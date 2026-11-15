// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/otfabric/canopenctl/internal/cli"
)

// padded returns a testdata file grown with comment lines to exactly size
// bytes.
func padded(t *testing.T, name string, size int) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	line := "; " + strings.Repeat("x", 1021) + "\n" // 1024 bytes
	var buf bytes.Buffer
	buf.Write(b)
	for buf.Len()+len(line) <= size {
		buf.WriteString(line)
	}
	if n := size - buf.Len(); n > 0 {
		buf.WriteString(";" + strings.Repeat("x", n-1))
	}
	if buf.Len() != size {
		t.Fatalf("padded to %d bytes, want %d", buf.Len(), size)
	}
	return buf.Bytes()
}

// allocated runs f and returns the bytes it (and anything running at the
// same time) allocated. TotalAlloc only grows, so no collection is needed
// first: a forced one takes seconds on the large heap of a long test run
// under -race (each in-process run keeps its command tree, through cobra's
// global map of flag completion functions).
func allocated(f func()) uint64 {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// TestEDSFileBound: EDS, DCF and concise DCF files are read up to
// cli.MaxFileSize, go-canopen's own bound for EDS and DCF files. A file of
// that size loads; a larger one is refused with exit 3 (invalid_eds)
// without being read: every command allocates far less than the file.
func TestEDSFileBound(t *testing.T) {
	fresh(t)
	ioNode(t, 5)
	dir := t.TempDir()
	big := filepath.Join(dir, "big.eds")
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(cli.MaxFileSize + 1); err != nil { // sparse: nothing written
		t.Fatal(err)
	}
	_ = f.Close()
	out := filepath.Join(dir, "out")
	for _, args := range [][]string{
		{"eds", "validate", big},
		{"eds", "show", big},
		{"eds", "convert", big, "--node-id", "5", "-o", out + ".dcf"},
		{"eds", "generate", big, "--package", "x", "-o", out + ".go"},
		{"dcf", "apply", "5", big, "--dry-run"},
		{"dcf", "export", "5", "--eds", big, "-i", "virtual", "-o", out + ".dcf"},
		{"cdcf", "show", big},
		{"cdcf", "show", "testdata/node37.dcf", "--eds", big},
		{"cdcf", "convert", big, "-o", out + ".cdcf"},
		{"cdcf", "apply", "5", big, "--dry-run"},
		{"boot", "--slave", "5=" + big, "-i", "virtual"},
		{"sdo", "read", "5", "0x1000", "0", "--eds", big, "-i", "virtual"},
		{"pdo", "show", "5", "--eds", big, "-i", "virtual"},
		{"simulate", "--eds", big, "--node-id", "5", "-i", "virtual", "--duration", "10ms"},
	} {
		var r result
		n := allocated(func() { r = run(t, "", append(args, "--format", "json")...) })
		want(t, r, 3, `"code": "invalid_eds"`, "file larger than 16777216 bytes")
		if n > 4<<20 {
			t.Errorf("%v allocated %d bytes for a file of %d bytes it refuses", args, n, cli.MaxFileSize+1)
		}
	}
	// A file of exactly the bound loads, as it did before.
	exact := filepath.Join(dir, "exact.eds")
	if err := os.WriteFile(exact, padded(t, "io401.eds", cli.MaxFileSize), 0o600); err != nil {
		t.Fatal(err)
	}
	want(t, run(t, "", "eds", "validate", exact), 0, "EDS valid")
	dcf := filepath.Join(dir, "exact.dcf")
	if err := os.WriteFile(dcf, padded(t, "node37.dcf", cli.MaxFileSize), 0o600); err != nil {
		t.Fatal(err)
	}
	want(t, run(t, "", "cdcf", "show", dcf), 0, "entries")
}
