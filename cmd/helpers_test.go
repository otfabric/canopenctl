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
	"sync"
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

// syncBuffer is an output stream of a command that the test reads while
// the command writes it.
type syncBuffer struct {
	mu      sync.Mutex
	b       bytes.Buffer
	changed chan struct{} // closed and replaced by every write
}

func newSyncBuffer() *syncBuffer { return &syncBuffer{changed: make(chan struct{})} }

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.b.Write(p)
	close(s.changed)
	s.changed = make(chan struct{})
	return n, err
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// state returns what was written so far and a channel the next write
// closes.
func (s *syncBuffer) state() (string, <-chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String(), s.changed
}

// bgCommand is a command running in the background.
type bgCommand struct {
	t           *testing.T
	args        []string
	cancel      context.CancelFunc
	out, errOut *syncBuffer
	done        chan struct{} // closed when Run returned
	code        int           // set before done is closed
}

// background runs a command concurrently and returns once the command has
// opened its interface (or ended before that). From then on every frame
// sent on the virtual bus reaches it: the port queues frames from its
// opening, and the command's bus reads them from its first subscription
// on (openBus), so the test can produce traffic at once.
func background(t *testing.T, args ...string) *bgCommand {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	c := &bgCommand{t: t, args: args, cancel: cancel, out: newSyncBuffer(), errOut: newSyncBuffer(), done: make(chan struct{})}
	t.Cleanup(cancel)
	opened := make(chan struct{})
	var once sync.Once
	orig := openTransport
	openTransport = func(iface string, o openOpts) (canbus.Transport, error) {
		tr, err := orig(iface, o)
		once.Do(func() { close(opened) })
		return tr, err
	}
	go func() {
		defer close(c.done)
		c.code = Run(ctx, args, strings.NewReader(""), c.out, c.errOut)
	}()
	select {
	case <-opened:
	case <-c.done:
	case <-time.After(10 * time.Second):
		// The wrapper stays: it only calls the original.
		t.Fatalf("%v did not open its interface within 10 s", args)
	}
	openTransport = orig // the command read it before closing opened or done
	return c
}

// waitOut waits until the command's stdout holds s. It fails the test
// when the command ends without writing it, or after 10 s.
func (c *bgCommand) waitOut(s string) {
	c.t.Helper()
	limit := time.NewTimer(10 * time.Second)
	defer limit.Stop()
	for {
		got, changed := c.out.state()
		if strings.Contains(got, s) {
			return
		}
		select {
		case <-changed:
		case <-c.done:
			if strings.Contains(c.out.String(), s) {
				return
			}
			c.t.Fatalf("%v ended (exit %d) without writing %q\nstdout: %s\nstderr: %s", c.args, c.code, s, c.out, c.errOut)
		case <-limit.C:
			c.t.Fatalf("%v did not write %q within 10 s\nstdout: %s\nstderr: %s", c.args, s, c.out, c.errOut)
		}
	}
}

// wait returns the result of the command once it ended by itself.
func (c *bgCommand) wait() result {
	c.t.Helper()
	select {
	case <-c.done:
	case <-time.After(30 * time.Second):
		c.cancel()
		c.t.Fatalf("%v did not end within 30 s\nstdout: %s\nstderr: %s", c.args, c.out, c.errOut)
	}
	return result{c.code, c.out.String(), c.errOut.String()}
}

// stop interrupts the command, as Ctrl-C does, and returns its result.
func (c *bgCommand) stop() result {
	c.t.Helper()
	c.cancel()
	return c.wait()
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
