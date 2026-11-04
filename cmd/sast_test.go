// SPDX-License-Identifier: MIT

package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestNodeIDFlagsDoNotWrap: integer node-ID flags were converted to a
// node-ID before their range check, so 257 became node-ID 1 and passed.
// Each is now refused with exit 3 before anything is sent.
func TestNodeIDFlagsDoNotWrap(t *testing.T) {
	fresh(t)
	ioNode(t, 1)
	for _, tc := range []struct {
		args []string
		msg  string
	}{
		{[]string{"sdo", "read", "1", "0x1000", "0", "--fd", "--client-id", "257"}, "--client-id 257"},
		{[]string{"sdo", "read", "1", "0x1000", "0", "--fd", "--client-id", "-255"}, "--client-id -255"},
		{[]string{"lss", "assign-all", "--first", "257"}, "--first 257"},
		{[]string{"lss", "assign-all", "--first", "0"}, "--first 0"},
		{[]string{"boot", "--slave", "1", "--master-id", "258"}, "--master-id 258"},
		{[]string{"simulate", "--node-id", "257", "--duration", "10ms"}, "--node-id 257"},
		{[]string{"simulate", "--node-id", "-1", "--duration", "10ms"}, "--node-id -1"},
	} {
		want(t, run(t, "", append(tc.args, "-i", "virtual")...), 3, tc.msg)
	}
}

// TestRangeFlags: --baud 1..9 was accepted although the help says
// 10..1000; --counter and --program keep their ranges.
func TestRangeFlags(t *testing.T) {
	fresh(t)
	out := filepath.Join(t.TempDir(), "n.dcf")
	for _, b := range []string{"5", "-1", "1001"} {
		want(t, run(t, "", "eds", "convert", "testdata/io401.eds", "--node-id", "5", "--baud", b, "-o", out), 3, "--baud "+b)
	}
	want(t, run(t, "", "eds", "convert", "testdata/io401.eds", "--node-id", "5", "--baud", "10", "-o", out), 0)
	want(t, run(t, "", "eds", "convert", "testdata/io401.eds", "--node-id", "5", "--baud", "1000", "-o", out), 0)
	for _, c := range []string{"1", "241", "-2"} {
		want(t, run(t, "", "sync", "--counter", c, "--count", "1", "-i", "virtual"), 3, "--counter "+c)
	}
	want(t, run(t, "", "sync", "--counter", "240", "--count", "1", "-i", "virtual"), 0)
	for _, p := range []string{"0", "255", "-1"} {
		want(t, run(t, "", "program", "status", "5", "--program", p, "-i", "virtual"), 3, "--program "+p)
	}
}

// TestMaxSize: --max-size bounds an upload over SDO (and USDO, through
// the same flags); a larger one is aborted locally with 0x05040005
// (exit 6, protocol: canopenctl refused, not the device) and the message
// names --max-size. Out-of-range values are refused.
func TestMaxSize(t *testing.T) {
	fresh(t)
	ioNode(t, 5)
	// 0x1008 is 26 bytes: a segmented upload.
	want(t, run(t, "", "sdo", "read", "5", "0x1008", "0", "-i", "virtual", "--max-size", "10"), 6, "0x05040005", "larger than --max-size")
	want(t, run(t, "", "sdo", "read", "5", "0x1008", "0", "-i", "virtual", "--max-size", "10", "--block"), 6, "0x05040005", "larger than --max-size")
	want(t, run(t, "", "sdo", "read", "5", "0x1008", "0", "-i", "virtual", "--max-size", "26"), 0, "OTF IO-401")
	for _, v := range []string{"0", "-1", "1073741825"} {
		want(t, run(t, "", "sdo", "read", "5", "0x1008", "0", "-i", "virtual", "--max-size", v), 3, "--max-size "+v)
	}
}

// TestOutputFileModes: files canopenctl writes are readable by their
// owner only (0600), directories it creates 0750.
func TestOutputFileModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes")
	}
	fresh(t)
	ioNode(t, 5)
	dir := t.TempDir()
	bin := filepath.Join(dir, "name.bin")
	want(t, run(t, "", "sdo", "read", "5", "0x1008", "0", "-i", "virtual", "-o", bin), 0)
	dcf := filepath.Join(dir, "n.dcf")
	want(t, run(t, "", "eds", "convert", "testdata/io401.eds", "--node-id", "5", "-o", dcf), 0)
	for _, f := range []string{bin, dcf} {
		st, err := os.Stat(f)
		if err != nil {
			t.Fatal(err)
		}
		if m := st.Mode().Perm(); m&0o077 != 0 {
			t.Errorf("%s: mode %v, want no group/other bits", f, m)
		}
	}
}
