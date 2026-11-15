// SPDX-License-Identifier: MIT

//go:build unix

package cmd

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// fifo makes a named pipe that serves content to its first reader: a
// file that can be read once.
func fifo(t *testing.T, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	go func() {
		f, err := os.OpenFile(path, os.O_WRONLY, 0) // waits for the reader
		if err != nil {
			return
		}
		_, _ = f.Write(content)
		_ = f.Close()
	}()
	t.Cleanup(func() {
		// A writer still waiting for a reader is released.
		if f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0); err == nil {
			_ = f.Close()
		}
	})
	return path
}

// runReadOnce runs a command on a file that can be read once (fifo). A
// command that opens it a second time waits for a writer that never
// comes: after 10 s the test gives that open an end of file and fails.
func runReadOnce(t *testing.T, path string, args ...string) result {
	t.Helper()
	done := make(chan result, 1)
	go func() { done <- runCtx(context.Background(), args...) }()
	select {
	case r := <-done:
		return r
	case <-time.After(10 * time.Second):
		if f, err := os.OpenFile(path, os.O_WRONLY, 0); err == nil {
			_ = f.Close()
		}
		r := <-done
		t.Fatalf("%v opened %s a second time (exit %d once released)\nstderr: %s", args, path, r.code, r.errOut)
		return r
	}
}

// TestConfigFilesReadOnce: cdcf show, cdcf apply and eds generate read
// their file once, and a file that is not a regular file (a pipe) is read
// as before, within the bound.
func TestConfigFilesReadOnce(t *testing.T) {
	fresh(t)
	dcf, err := os.ReadFile("testdata/node37.dcf")
	if err != nil {
		t.Fatal(err)
	}
	io401, err := os.ReadFile("testdata/io401.eds")
	if err != nil {
		t.Fatal(err)
	}
	p := fifo(t, "node37.dcf", dcf)
	want(t, runReadOnce(t, p, "cdcf", "show", p), 0, "0x1017:00", "Producer heartbeat time", "entries")
	p = fifo(t, "node37.dcf", dcf)
	want(t, runReadOnce(t, p, "cdcf", "apply", "37", p, "--dry-run"), 0, "Producer heartbeat time", "dry run")
	p = fifo(t, "io401.eds", io401)
	want(t, runReadOnce(t, p, "eds", "generate", p, "--package", "io401", "-o", filepath.Join(t.TempDir(), "io401.go")), 0, "go")
	p = fifo(t, "io401.eds", io401)
	want(t, runReadOnce(t, p, "eds", "validate", p), 0, "EDS valid")
}

// TestEDSFileEndless: a device that never ends is read up to the bound,
// then refused (exit 3).
func TestEDSFileEndless(t *testing.T) {
	for _, args := range [][]string{
		{"eds", "validate", "/dev/zero"},
		{"cdcf", "show", "/dev/zero"},
	} {
		want(t, run(t, "", args...), 3, "file larger than 16777216 bytes")
	}
}
