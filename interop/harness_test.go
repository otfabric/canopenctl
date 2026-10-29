// SPDX-License-Identifier: MIT

//go:build interop

package interop

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// InteropRelease is the canopen-interop release whose images the suite
// runs (contract 1.0). Override the tag with CANOPENCTL_INTEROP_TAG (dev
// for locally built images) or one image with
// CANOPENCTL_INTEROP_<ADAPTER>_IMAGE (upper case).
const InteropRelease = "v0.1.0"

// adapters are the canopen-interop images.
var adapters = []string{"canopennode", "lely", "pythoncanopen"}

func image(adapter string) string {
	if v := os.Getenv("CANOPENCTL_INTEROP_" + strings.ToUpper(adapter) + "_IMAGE"); v != "" {
		return v
	}
	tag := os.Getenv("CANOPENCTL_INTEROP_TAG")
	if tag == "" {
		tag = InteropRelease
	}
	return "ghcr.io/otfabric/canopen-interop-" + adapter + ":" + tag
}

func dockerCmd() []string {
	if v := os.Getenv("CANOPENCTL_INTEROP_DOCKER"); v != "" {
		return strings.Fields(v)
	}
	return []string{"docker"}
}

func require() bool { return os.Getenv("CANOPENCTL_REQUIRE_INTEROP") == "1" }

// skipf skips loudly, or fails with CANOPENCTL_REQUIRE_INTEROP=1.
func skipf(t *testing.T, format string, args ...any) {
	t.Helper()
	if require() {
		t.Fatalf("SKIP not allowed (CANOPENCTL_REQUIRE_INTEROP=1): "+format, args...)
	}
	t.Skipf("SKIP: "+format, args...)
}

// needVCAN returns $CANOPENCTL_TEST_IFACE (default vcan0).
func needVCAN(t *testing.T) string {
	t.Helper()
	name := os.Getenv("CANOPENCTL_TEST_IFACE")
	if name == "" {
		name = "vcan0"
	}
	ifi, err := net.InterfaceByName(name)
	if err != nil || ifi.Flags&net.FlagUp == 0 {
		if os.Getenv("CANOPENCTL_REQUIRE_VCAN") == "1" || require() {
			t.Fatalf("%s missing or down", name)
		}
		t.Skipf("SKIP: %s missing or down", name)
	}
	return name
}

var bin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "canopenctl-interop")
	if err != nil {
		panic(err)
	}
	bin = filepath.Join(dir, "canopenctl")
	if v := os.Getenv("CANOPENCTL_BIN"); v != "" {
		bin = v
	} else if out, err := exec.Command("go", "build", "-o", bin, "..").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build: %v\n%s", err, out)
		os.Exit(2)
	}
	code := m.Run()
	writeReport()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// The report: one row per adapter and scenario.
type row struct {
	Adapter  string `json:"adapter"`
	Scenario string `json:"scenario"`
	Result   string `json:"result"` // pass, fail, skip, known-difference
	Detail   string `json:"detail,omitempty"`
}

var (
	reportMu sync.Mutex
	report   []row
)

func record(adapter, scenario, result, detail string) {
	reportMu.Lock()
	report = append(report, row{adapter, scenario, result, detail})
	reportMu.Unlock()
}

func writeReport() {
	path := os.Getenv("CANOPENCTL_INTEROP_REPORT")
	if path == "" {
		return
	}
	reportMu.Lock()
	defer reportMu.Unlock()
	sort.SliceStable(report, func(i, j int) bool { return report[i].Adapter < report[j].Adapter })
	imgs := map[string]string{}
	for _, a := range adapters {
		imgs[a] = image(a)
	}
	b, _ := json.MarshalIndent(map[string]any{"interopRelease": InteropRelease, "images": imgs, "results": report}, "", "  ")
	_ = os.WriteFile(path, b, 0o644)
}

// knownDiffs are the documented differences scenarios report (by test).
var knownDiffs sync.Map

// knownDifference ends a scenario as a documented difference between
// canopenctl and a stack: recorded as such, never as a pass.
func knownDifference(t *testing.T, detail string) {
	t.Helper()
	knownDiffs.Store(t, detail)
	t.Logf("known difference: %s", detail)
}

// scenario runs one check of an adapter and records its outcome.
func scenario(t *testing.T, adapter, name string, f func(t *testing.T)) {
	t.Run(name, func(t *testing.T) {
		defer func() {
			if d, ok := knownDiffs.Load(t); ok && !t.Failed() {
				record(adapter, name, "known-difference", d.(string))
				return
			}
			switch {
			case t.Skipped():
				record(adapter, name, "skip", "")
			case t.Failed():
				record(adapter, name, "fail", "")
			default:
				record(adapter, name, "pass", "")
			}
		}()
		f(t)
	})
}

// caps is an image's capability document.
type caps struct {
	Operations []string        `json:"operations"`
	Features   map[string]bool `json:"features"`
}

func (c caps) op(o string) bool {
	for _, x := range c.Operations {
		if x == o {
			return true
		}
	}
	return false
}

var capsCache sync.Map

// needImage makes sure the image is there (pulling it once) and returns
// its capabilities; a missing image is a skip.
func needImage(t *testing.T, adapter string) caps {
	t.Helper()
	if c, ok := capsCache.Load(adapter); ok {
		return c.(caps)
	}
	img := image(adapter)
	d := dockerCmd()
	if err := exec.Command(d[0], append(d[1:], "image", "inspect", img)...).Run(); err != nil {
		if out, err := exec.Command(d[0], append(d[1:], "pull", "-q", img)...).CombinedOutput(); err != nil {
			skipf(t, "image %s not available: %s", img, strings.TrimSpace(string(out)))
		}
	}
	out, err := exec.Command(d[0], append(d[1:], "run", "--rm", img, "print-capabilities")...).Output()
	if err != nil {
		t.Fatalf("%s print-capabilities: %v", img, err)
	}
	var c caps
	if err := json.Unmarshal(lastLine(out), &c); err != nil {
		t.Fatalf("%s capabilities: %v\n%s", img, err, out)
	}
	capsCache.Store(adapter, c)
	return c
}

func lastLine(b []byte) []byte {
	lines := bytes.Split(bytes.TrimSpace(b), []byte("\n"))
	return lines[len(lines)-1]
}

// fixture copies a fixture of the image (relative to /fixtures) into dir.
func fixture(t *testing.T, adapter, rel, dir string) string {
	t.Helper()
	d := dockerCmd()
	out, err := exec.Command(d[0], append(d[1:], "run", "--rm", image(adapter), "print-fixture", rel)...).Output()
	if err != nil {
		t.Fatalf("print-fixture %s: %v", rel, err)
	}
	p := filepath.Join(dir, filepath.Base(rel))
	if err := os.WriteFile(p, out, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// container is an adapter running in the background.
type container struct {
	name string
	cmd  *exec.Cmd
	mu   sync.Mutex
	out  bytes.Buffer
	errb bytes.Buffer
	line chan string
	done chan struct{}
}

func (c *container) stdout() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.out.String()
}

func randName() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "canopenctl-interop-" + hex.EncodeToString(b)
}

// startContainer runs an adapter command in the background on the host
// network (CAN interfaces belong to the host's network namespace).
func startContainer(t *testing.T, adapter string, args ...string) *container {
	t.Helper()
	d := dockerCmd()
	c := &container{name: randName(), line: make(chan string, 1024), done: make(chan struct{})}
	full := append(append([]string{}, d[1:]...), "run", "--rm", "--name", c.name, "--network", "host", image(adapter))
	c.cmd = exec.Command(d[0], append(full, args...)...)
	pipe, err := c.cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	c.cmd.Stderr = &c.errb
	if err := c.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		defer close(c.done)
		s := bufio.NewScanner(io.Reader(pipe))
		for s.Scan() {
			c.mu.Lock()
			c.out.WriteString(s.Text() + "\n")
			c.mu.Unlock()
			select {
			case c.line <- s.Text():
			default:
			}
		}
	}()
	t.Cleanup(func() {
		_ = exec.Command(d[0], append(d[1:], "kill", c.name)...).Run()
		<-c.done
		_ = c.cmd.Wait()
	})
	return c
}

func (c *container) await(t *testing.T, s string, d time.Duration) {
	t.Helper()
	deadline := time.After(d)
	for {
		select {
		case l := <-c.line:
			if strings.Contains(l, s) {
				return
			}
		case <-c.done:
			t.Fatalf("container ended before %q\nstdout: %s\nstderr: %s", s, c.stdout(), c.errb.String())
		case <-deadline:
			t.Fatalf("no %q within %v\nstdout: %s\nstderr: %s", s, d, c.stdout(), c.errb.String())
		}
	}
}

// stop sends SIGTERM (docker stop) and waits for the container's result.
func (c *container) stop(t *testing.T) map[string]any {
	t.Helper()
	d := dockerCmd()
	_ = exec.Command(d[0], append(d[1:], "stop", "-t", "5", c.name)...).Run()
	select {
	case <-c.done:
	case <-time.After(20 * time.Second):
		t.Fatal("container did not stop")
	}
	return lastDoc(t, c.stdout())
}

// wait waits for a container that ends by itself.
func (c *container) wait(t *testing.T, d time.Duration) (int, map[string]any) {
	t.Helper()
	select {
	case <-c.done:
	case <-time.After(d):
		t.Fatalf("container did not end within %v\nstdout: %s\nstderr: %s", d, c.stdout(), c.errb.String())
	}
	_ = c.cmd.Wait()
	return c.cmd.ProcessState.ExitCode(), lastDoc(t, c.stdout())
}

func lastDoc(t *testing.T, s string) map[string]any {
	t.Helper()
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(lastLine([]byte(s)), &m); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, s)
	}
	return m
}

// adapterRun runs an adapter command to its end.
func adapterRun(t *testing.T, adapter string, args ...string) (int, string) {
	t.Helper()
	d := dockerCmd()
	full := append(append([]string{}, d[1:]...), "run", "--rm", "--network", "host", image(adapter))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, d[0], append(full, args...)...)
	var o, e bytes.Buffer
	cmd.Stdout, cmd.Stderr = &o, &e
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return code, o.String()
}

type result struct {
	code     int
	out, err string
}

// ctl runs canopenctl to its end.
func ctl(t *testing.T, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	var o, e bytes.Buffer
	cmd.Stdout, cmd.Stderr = &o, &e
	err := cmd.Run()
	r := result{out: o.String(), err: e.String()}
	if ee, ok := err.(*exec.ExitError); ok {
		r.code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return r
}

func want(t *testing.T, r result, code int, contains ...string) {
	t.Helper()
	if r.code != code {
		t.Fatalf("exit %d, want %d\nstdout: %s\nstderr: %s", r.code, code, r.out, r.err)
	}
	for _, c := range contains {
		if !strings.Contains(r.out+r.err, c) {
			t.Fatalf("output lacks %q\nstdout: %s\nstderr: %s", c, r.out, r.err)
		}
	}
}

// ctlProc is canopenctl running in the background.
type ctlProc struct {
	cmd  *exec.Cmd
	mu   sync.Mutex
	out  bytes.Buffer
	errb bytes.Buffer
	line chan string
	done chan struct{}
}

func (p *ctlProc) stdout() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.out.String()
}

func startCtl(t *testing.T, args ...string) *ctlProc {
	t.Helper()
	p := &ctlProc{cmd: exec.Command(bin, args...), line: make(chan string, 4096), done: make(chan struct{})}
	pipe, err := p.cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	p.cmd.Stderr = &p.errb
	if err := p.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		defer close(p.done)
		s := bufio.NewScanner(pipe)
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
	t.Cleanup(func() { p.stop() })
	return p
}

func (p *ctlProc) await(t *testing.T, s string, d time.Duration) {
	t.Helper()
	deadline := time.After(d)
	for {
		select {
		case l := <-p.line:
			if strings.Contains(l, s) {
				return
			}
		case <-p.done:
			t.Fatalf("canopenctl ended before %q\nstdout: %s\nstderr: %s", s, p.stdout(), p.errb.String())
		case <-deadline:
			t.Fatalf("no %q within %v\nstdout: %s\nstderr: %s", s, d, p.stdout(), p.errb.String())
		}
	}
}

func (p *ctlProc) stop() int {
	if p.cmd.ProcessState == nil {
		_ = p.cmd.Process.Signal(syscall.SIGINT)
		select {
		case <-p.done:
		case <-time.After(10 * time.Second):
			_ = p.cmd.Process.Kill()
			<-p.done
		}
		_ = p.cmd.Wait()
	}
	return p.cmd.ProcessState.ExitCode()
}

func (p *ctlProc) wait(t *testing.T, d time.Duration) result {
	t.Helper()
	select {
	case <-p.done:
	case <-time.After(d):
		_ = p.cmd.Process.Kill()
		t.Fatalf("canopenctl did not end within %v\nstdout: %s\nstderr: %s", d, p.stdout(), p.errb.String())
	}
	_ = p.cmd.Wait()
	return result{code: p.cmd.ProcessState.ExitCode(), out: p.stdout(), err: p.errb.String()}
}
