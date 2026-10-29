// SPDX-License-Identifier: MIT

//go:build linux

package e2e

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// vcan0 returns $CANOPENCTL_TEST_IFACE (default vcan0), a vcan interface
// that is up; missing, it fails with CANOPENCTL_REQUIRE_VCAN=1 and skips
// loudly otherwise.
func vcan0(t *testing.T) string {
	t.Helper()
	name := os.Getenv("CANOPENCTL_TEST_IFACE")
	if name == "" {
		name = "vcan0"
	}
	ifi, err := net.InterfaceByName(name)
	if err != nil || ifi.Flags&net.FlagUp == 0 {
		if os.Getenv("CANOPENCTL_REQUIRE_VCAN") == "1" {
			t.Fatalf("no usable %s (CANOPENCTL_REQUIRE_VCAN=1)", name)
		}
		t.Skipf("SKIP: no usable %s: sudo ip link add %s type vcan && sudo ip link set %s up", name, name, name)
	}
	return name
}

// proc is a canopenctl process running in the background.
type proc struct {
	cmd  *exec.Cmd
	mu   sync.Mutex
	out  bytes.Buffer
	err  bytes.Buffer
	done chan struct{}
	line chan string
}

func (p *proc) stdout() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.out.String()
}

// start runs canopenctl in the background; its stdout lines are also
// delivered on p.line.
func start(t *testing.T, args ...string) *proc {
	t.Helper()
	p := &proc{cmd: exec.Command(bin, args...), done: make(chan struct{}), line: make(chan string, 4096)}
	pipe, err := p.cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	p.cmd.Stderr = &p.err
	if err := p.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		defer close(p.done)
		s := bufio.NewScanner(io.Reader(pipe))
		for s.Scan() {
			p.mu.Lock()
			p.out.WriteString(s.Text() + "\n")
			p.mu.Unlock()
			select {
			case p.line <- s.Text():
			default:
			}
		}
	}()
	t.Cleanup(func() { p.stop(t) })
	return p
}

// await waits for a stdout line containing s.
func (p *proc) await(t *testing.T, s string, d time.Duration) string {
	t.Helper()
	deadline := time.After(d)
	for {
		select {
		case l := <-p.line:
			if strings.Contains(l, s) {
				return l
			}
		case <-p.done:
			t.Fatalf("process ended before %q\nstdout: %s\nstderr: %s", s, p.stdout(), p.err.String())
		case <-deadline:
			t.Fatalf("no %q within %v\nstdout: %s\nstderr: %s", s, d, p.stdout(), p.err.String())
		}
	}
}

// stop interrupts the process (SIGINT, as Ctrl-C) and returns its exit
// code.
func (p *proc) stop(t *testing.T) int {
	t.Helper()
	if p.cmd.ProcessState != nil {
		return p.cmd.ProcessState.ExitCode()
	}
	_ = p.cmd.Process.Signal(syscall.SIGINT)
	select {
	case <-p.done:
	case <-time.After(10 * time.Second):
		_ = p.cmd.Process.Kill()
		<-p.done
	}
	_ = p.cmd.Wait()
	return p.cmd.ProcessState.ExitCode()
}

// wait waits for the process to end by itself.
func (p *proc) wait(t *testing.T, d time.Duration) result {
	t.Helper()
	select {
	case <-p.done:
	case <-time.After(d):
		_ = p.cmd.Process.Kill()
		t.Fatalf("process did not end within %v\nstdout: %s\nstderr: %s", d, p.stdout(), p.err.String())
	}
	_ = p.cmd.Wait()
	return result{code: p.cmd.ProcessState.ExitCode(), out: p.stdout(), err: p.err.String()}
}

// TestVCANNode runs a simulated CiA 401 node as a process on vcan0 and
// operates it with every command, each in its own process.
func TestVCANNode(t *testing.T) {
	ifc := vcan0(t)
	const id = "21"
	sim := start(t, "simulate", "-i", ifc, "--eds", testdata("io401.eds"), "--node-id", id, "--heartbeat", "50",
		"--tick", "6000:01", "--tick-period", "20ms", "--emcy", "2310:03:0102", "--emcy-after", "1500ms", "--format", "json")
	sim.await(t, `"type":"ready"`, 10*time.Second)

	// Passive watchers first: they must hear the emergency and the state changes.
	emcyW := start(t, "emcy", "watch", "-i", ifc, "--node", id, "--count", "1", "--format", "json")
	hb := start(t, "heartbeat", "watch", "-i", ifc, "--node", id, "--supervise", "500ms", "--duration", "60s")
	mon := start(t, "monitor", "-i", ifc, "--node", id, "--service", "sdo", "--count", "4", "--format", "json")

	want(t, run(t, "", "scan", "-i", ifc, "--nodes", "16-24"), 0, "node  21", "CiA 401 generic I/O", "vendor 0x00000F7A")
	want(t, run(t, "", "sdo", "read", id, "0x1018", "1", "-i", ifc), 0, "3962 (0x00000F7A)")
	want(t, run(t, "", "sdo", "write", id, "0x6200", "1", "0x5A", "--eds", testdata("io401.eds"), "-i", ifc, "--verify"), 0, "verified")
	want(t, run(t, "", "sdo", "read", id, "0x1008", "0", "-i", ifc, "--block"), 0, "OTF IO-401")
	want(t, run(t, "", "sdo", "dump", id, "--eds", testdata("io401.eds"), "-i", ifc, "--format", "table"), 0, "0x6200:01", "0x5A")
	want(t, run(t, "", "sdo", "read", id, "0x5FFF", "0", "-i", ifc), 11, "0x06020000")

	r := emcyW.wait(t, 10*time.Second)
	want(t, r, 0, `"code":"0x2310"`, `"node":21`)
	r = mon.wait(t, 10*time.Second)
	want(t, r, 0, `"kind":"sdo-request"`, `"kind":"sdo-response"`)

	want(t, run(t, "", "nmt", "start", id, "-i", ifc, "--wait"), 0, "reports operational")
	want(t, run(t, "", "pdo", "show", id, "-i", ifc, "--eds", testdata("io401.eds")), 0, "TPDO1  COB-ID 0x195 (valid)  active")
	want(t, run(t, "", "pdo", "watch", id, "--tpdo", "1", "--eds", testdata("io401.eds"), "-i", ifc, "--count", "2"), 0, "TPDO1 195#", "Read input 1h to 1h+7=")
	want(t, run(t, "", "nmt", "preop", id, "-i", ifc, "--wait"), 0)
	want(t, run(t, "", "pdo", "map", id, "--tpdo", "2", "--map", "6000:01:8,6000:02:8", "--type", "255", "--event-timer", "20", "-i", ifc), 0, "TPDO2  COB-ID 0x295 (valid)  active")
	want(t, run(t, "", "nmt", "start", id, "-i", ifc, "--wait"), 0)
	want(t, run(t, "", "pdo", "watch", id, "--tpdo", "2", "-i", ifc, "--cob-id", "295", "--map", "6000:01:8,6000:02:8", "--count", "2"), 0, "TPDO2 295#")

	syncMon := start(t, "monitor", "-i", ifc, "--service", "sync", "--count", "3")
	time.Sleep(200 * time.Millisecond)
	want(t, run(t, "", "sync", "-i", ifc, "--count", "3", "--period", "5ms"), 0, "3 SYNC sent")
	want(t, syncMon.wait(t, 10*time.Second), 0, "SYNC")
	want(t, run(t, "", "time", "-i", ifc), 0, "TIME message")

	dir := t.TempDir()
	dcf := filepath.Join(dir, "node21.dcf")
	want(t, run(t, "", "dcf", "export", id, "--eds", testdata("io401.eds"), "-o", dcf, "-i", ifc), 0, "0 failed")
	want(t, run(t, "", "eds", "validate", dcf), 0)
	want(t, run(t, "", "nmt", "preop", id, "-i", ifc, "--wait"), 0)
	want(t, run(t, "", "dcf", "apply", id, dcf, "-i", ifc), 0, "written to node 21")
	want(t, run(t, "", "sdo", "read", id, "0x6200", "1", "-i", ifc), 0, "90 (0x5A)")
	want(t, run(t, "", "nmt", "reset-node", id, "-i", ifc, "--wait"), 0, "boot-up")

	time.Sleep(300 * time.Millisecond)
	if code := hb.stop(t); code != 0 {
		t.Fatalf("heartbeat watch exit %d\n%s\n%s", code, hb.stdout(), hb.err.String())
	}
	for _, w := range []string{"node 21 pre-operational -> operational", "node 21 boot-up"} {
		if !strings.Contains(hb.stdout(), w) {
			t.Fatalf("heartbeat watch lacks %q:\n%s", w, hb.stdout())
		}
	}
	if code := sim.stop(t); code != 0 {
		t.Fatalf("simulate exit %d\n%s", code, sim.err.String())
	}
	out := sim.stdout()
	for _, w := range []string{`"type":"state"`, `"command":"start"`, `"type":"summary"`} {
		if !strings.Contains(out, w) {
			t.Fatalf("simulate output lacks %s:\n%s", w, out)
		}
	}
}

// TestVCANDrive operates a simulated CiA 402 drive process.
func TestVCANDrive(t *testing.T) {
	ifc := vcan0(t)
	sim := start(t, "simulate", "-i", ifc, "--node-id", "22", "--drive", "--heartbeat", "0")
	sim.await(t, "node 22 running", 10*time.Second)
	want(t, run(t, "", "cia402", "state", "22", "-i", ifc), 0, "node 22: ")
	want(t, run(t, "", "cia402", "enable", "22", "-i", ifc), 0, "Operation enabled")
	want(t, run(t, "", "cia402", "move", "22", "5000", "--wait", "--velocity", "200000", "--accel", "2000000", "--decel", "2000000", "-i", ifc), 0, "position 5000")
	want(t, run(t, "", "cia402", "velocity", "22", "1000", "-i", ifc), 0, "PV")
	want(t, run(t, "", "cia402", "quick-stop", "22", "-i", ifc), 0)
	want(t, run(t, "", "cia402", "disable", "22", "-i", ifc, "--format", "json"), 0, `"state": "Switch on disabled"`)
	if code := sim.stop(t); code != 0 {
		t.Fatalf("simulate exit %d", code)
	}
}

// TestVCANSafety: vcan needs no --yes; a non-virtual interface does, and
// nothing is sent then.
func TestVCANSafety(t *testing.T) {
	ifc := vcan0(t)
	mon := start(t, "monitor", "-i", ifc, "--service", "nmt", "--duration", "1s", "--format", "json")
	time.Sleep(200 * time.Millisecond)
	want(t, run(t, "", "nmt", "stop", "all", "-i", "slcan:/dev/null"), 9, "--yes")
	want(t, run(t, "", "nmt", "stop", "99", "-i", ifc), 0)
	r := mon.wait(t, 10*time.Second)
	want(t, r, 0, `"target":99`)
	if strings.Count(r.out, `"kind":"nmt"`) != 1 {
		t.Fatalf("expected one NMT command:\n%s", r.out)
	}
}
