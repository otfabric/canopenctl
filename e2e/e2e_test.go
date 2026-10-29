// SPDX-License-Identifier: MIT

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

var bin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "canopenctl-e2e")
	if err != nil {
		panic(err)
	}
	bin = filepath.Join(dir, "canopenctl")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if v := os.Getenv("CANOPENCTL_BIN"); v != "" {
		bin = v // a prebuilt binary (a VM without a Go toolchain)
	} else if out, err := exec.Command("go", "build", "-race", "-o", bin, "..").CombinedOutput(); err != nil {
		// -race needs cgo; fall back to a plain build.
		if out2, err2 := exec.Command("go", "build", "-o", bin, "..").CombinedOutput(); err2 != nil {
			fmt.Fprintf(os.Stderr, "build: %v\n%s\n%s", err2, out, out2)
			os.Exit(2)
		}
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

type result struct {
	code     int
	out, err string
}

func run(t *testing.T, stdin string, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = strings.NewReader(stdin)
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

func doc(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, s)
	}
	return m
}

func testdata(name string) string { return filepath.Join("..", "cmd", "testdata", name) }

// TestEDSChain: EDS → DCF for a node → validate → show (node-ID values
// evaluated) → back to an EDS → validate → Go code.
func TestEDSChain(t *testing.T) {
	dir := t.TempDir()
	dcf := filepath.Join(dir, "node9.dcf")
	want(t, run(t, "", "eds", "validate", testdata("io401.eds")), 0, "EDS valid")
	want(t, run(t, "", "eds", "convert", testdata("io401.eds"), "--node-id", "9", "-o", dcf), 0)
	want(t, run(t, "", "eds", "validate", dcf), 0, "DCF valid")
	r := run(t, "", "eds", "show", dcf, "--range", "1800", "--format", "json")
	want(t, r, 0)
	objs := doc(t, r.out)["result"].(map[string]any)["objects"].([]any)
	found := false
	for _, o := range objs {
		m := o.(map[string]any)
		if m["key"] == "0x1800:01" {
			found = m["default"] == "0x00000189"
		}
	}
	if !found {
		t.Fatalf("0x1800:01 of node 9:\n%s", r.out)
	}
	back := filepath.Join(dir, "back.eds")
	want(t, run(t, "", "eds", "convert", dcf, "-o", back), 0)
	want(t, run(t, "", "eds", "validate", back, "--strict"), 0)
	gen := filepath.Join(dir, "io401", "io401.go")
	want(t, run(t, "", "eds", "generate", back, "--package", "io401", "-o", gen), 0)
	if b, _ := os.ReadFile(gen); !bytes.Contains(b, []byte("func NewDictionary(")) {
		t.Fatal("generated code")
	}
	want(t, run(t, "", "eds", "validate", testdata("broken.dcf")), 1, "DCF invalid")
}

// TestDecodeStdin decodes a trace piped in.
func TestDecodeStdin(t *testing.T) {
	trace := "(1.000000) vcan0 000#0105\n(1.001000) vcan0 705#7F\n(1.002000) vcan0 085#1023010102030000\n"
	r := run(t, trace, "decode", "-", "--format", "json")
	want(t, r, 0, `"kind":"nmt"`, `"kind":"heartbeat"`, `"codeText":"current, device output side (0x2310)"`, `"type":"summary"`)
}

// TestErrorDocuments: exit codes and JSON error documents of the binary.
func TestErrorDocuments(t *testing.T) {
	r := run(t, "", "bogus", "--format", "json")
	want(t, r, 2)
	if doc(t, r.out)["error"].(map[string]any)["kind"] != "usage" {
		t.Fatal(r.out)
	}
	r = run(t, "", "sdo", "write", "5", "1017", "0", "1", "-t", "u16", "-i", "slcan:/dev/null", "--format", "json")
	want(t, r, 9, `"code": "confirmation_required"`)
	want(t, run(t, "", "sdo", "read", "500", "1000", "0", "-i", "virtual"), 3, "invalid node-ID")
	want(t, run(t, "", "sdo", "read", "5", "1000", "0", "-i", "virtual", "--timeout", "20ms"), 5, "timeout")
	want(t, run(t, "", "version"), 0, "canopenctl", "go-canopen")
}

// TestSimulateVirtual: a node on the process's own virtual bus starts,
// reports ready and ends at --duration.
func TestSimulateVirtual(t *testing.T) {
	r := run(t, "", "simulate", "-i", "virtual", "--node-id", "5", "--duration", "200ms", "--format", "json")
	want(t, r, 0, `"type":"ready"`, `"type":"summary"`)
}
