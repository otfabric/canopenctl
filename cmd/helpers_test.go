// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/otfabric/go-canbus"
	"github.com/otfabric/go-canbus/virtual"
	"github.com/otfabric/go-canopen"
	"github.com/otfabric/go-canopen/eds"
	"github.com/otfabric/go-canopen/lss"
	"github.com/otfabric/go-canopen/node"
	"github.com/otfabric/go-canopen/od"
)

var update = flag.Bool("update", false, "rewrite the golden files")

type result struct {
	code        int
	out, errOut string
}

// fresh gives the test a new in-process virtual bus.
func fresh(t *testing.T) {
	t.Helper()
	resetVirtual()
	t.Cleanup(resetVirtual)
}

func resetVirtual() {
	virtualMu.Lock()
	defer virtualMu.Unlock()
	if virtualBus != nil {
		_ = virtualBus.Close()
	}
	virtualBus = nil
}

// run executes canopenctl in process on the current virtual bus.
func run(t *testing.T, stdin string, args ...string) result {
	t.Helper()
	var out, errOut bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	code := Run(ctx, args, strings.NewReader(stdin), &out, &errOut)
	if ctx.Err() != nil {
		t.Fatalf("%v ran into the test's 30 s limit\nstdout: %s\nstderr: %s", args, out.String(), errOut.String())
	}
	return result{code, out.String(), errOut.String()}
}

// runCtx runs canopenctl with a context the test controls.
func runCtx(ctx context.Context, args ...string) result {
	var out, errOut bytes.Buffer
	code := Run(ctx, args, strings.NewReader(""), &out, &errOut)
	return result{code, out.String(), errOut.String()}
}

// background runs a command concurrently; wait returns its result.
func background(args ...string) (wait func() result) {
	ch := make(chan result, 1)
	go func() { ch <- runCtx(context.Background(), args...) }()
	return func() result {
		select {
		case r := <-ch:
			return r
		case <-time.After(30 * time.Second):
			return result{code: -1, errOut: "background command did not end"}
		}
	}
}

// port opens a bus on the shared virtual bus, closed with the test.
func port(t *testing.T) *canbus.Bus {
	t.Helper()
	p, err := sharedVirtual().Open()
	if err != nil {
		t.Fatal(err)
	}
	b, err := canbus.NewBus(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}

// fdPort opens a CAN FD bus on the shared virtual bus, closed with the
// test.
func fdPort(t *testing.T) *canbus.Bus {
	t.Helper()
	p, err := sharedVirtual().Open(virtual.WithFD())
	if err != nil {
		t.Fatal(err)
	}
	b, err := canbus.NewBus(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}

// fdNode starts a CANopen FD node (node.WithFD) on a CAN FD port of the
// virtual bus.
func fdNode(t *testing.T, d *od.Dictionary, opts ...node.Option) *node.Node {
	t.Helper()
	n, err := node.New(fdPort(t), d, append(opts, node.WithFD(false))...)
	if err != nil {
		t.Fatal(err)
	}
	if err := n.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = n.Close() })
	return n
}

// lssNode starts an LSS slave with simulate's built-in dictionary and
// serial number serial: unconfigured with id 0.
func lssNode(t *testing.T, id canopen.NodeID, serial uint32, opts ...node.Option) *node.Node {
	t.Helper()
	if id == 0 {
		id = lss.Unconfigured
	}
	d, err := builtinDictionarySerial(id, false, serial)
	if err != nil {
		t.Fatal(err)
	}
	return simNode(t, d, append(opts, node.WithLSS())...)
}

// lssAddr is the LSS address of a built-in node with a serial number.
func lssAddr(serial uint32) string {
	return lss.Address{Vendor: 0x00000F7A, Product: 0x0000C0C7, Revision: 0x00010000, Serial: serial}.String()
}

// simNode starts a node with a dictionary on the virtual bus.
func simNode(t *testing.T, d *od.Dictionary, opts ...node.Option) *node.Node {
	t.Helper()
	n, err := node.New(port(t), d, opts...)
	if err != nil {
		t.Fatal(err)
	}
	if err := n.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = n.Close() })
	return n
}

// edsDict builds the dictionary of a testdata EDS for a node.
func edsDict(t *testing.T, file string, id canopen.NodeID) *od.Dictionary {
	t.Helper()
	f, err := eds.ParseFile(filepath.Join("testdata", file))
	if err != nil {
		t.Fatal(err)
	}
	d, _, err := f.BuildDictionary(id, eds.BuildOptions{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// ioNode starts node id with testdata/io401.eds.
func ioNode(t *testing.T, id canopen.NodeID, opts ...node.Option) *node.Node {
	t.Helper()
	return simNode(t, edsDict(t, "io401.eds", id), opts...)
}

// builtinNode starts node id with simulate's built-in dictionary.
func builtinNode(t *testing.T, id canopen.NodeID, drive bool, opts ...node.Option) *node.Node {
	t.Helper()
	d, err := builtinDictionary(id, drive)
	if err != nil {
		t.Fatal(err)
	}
	return simNode(t, d, opts...)
}

func want(t *testing.T, r result, code int, contains ...string) {
	t.Helper()
	if r.code != code {
		t.Fatalf("exit %d, want %d\nstdout: %s\nstderr: %s", r.code, code, r.out, r.errOut)
	}
	all := r.out + r.errOut
	for _, c := range contains {
		if !strings.Contains(all, c) {
			t.Fatalf("output lacks %q\nstdout: %s\nstderr: %s", c, r.out, r.errOut)
		}
	}
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)
	if *update {
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test ./cmd -update)", err)
	}
	if strings.ReplaceAll(string(b), "\r\n", "\n") != got {
		t.Fatalf("%s differs:\n--- got\n%s\n--- want\n%s", name, got, b)
	}
}

// eventually polls cond for up to 5 s.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func linux() bool { return runtime.GOOS == "linux" }
