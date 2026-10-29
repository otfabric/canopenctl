// SPDX-License-Identifier: MIT

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otfabric/go-canopen"
)

func TestSDORead(t *testing.T) {
	fresh(t)
	ioNode(t, 5)
	want(t, run(t, "", "sdo", "read", "5", "0x1018", "1", "-i", "virtual"), 0, "node 5 0x1018:01 (UNSIGNED32) = 3962 (0x00000F7A)")
	want(t, run(t, "", "sdo", "read", "5", "1018", "0x01", "-i", "virtual", "--eds", "testdata/io401.eds"), 0, "Identity object / Vendor-ID")
	r := run(t, "", "sdo", "read", "5", "0x1008", "0", "-i", "virtual", "--format", "json")
	want(t, r, 0, `"typeSource": "guess"`, `"type": "VISIBLE_STRING"`, `"command": "sdo read"`)
	want(t, run(t, "", "sdo", "read", "5", "0x1008", "0", "-i", "virtual", "--type", "string", "--block"), 0, `"OTF IO-401 8DI/8DO 2AI/2AO"`)
	want(t, run(t, "", "sdo", "read", "5", "0x1017", "0", "-i", "virtual", "-t", "i16", "--format", "table"), 0, "NODE", "INTEGER16")
	// The wrong type for the data: 2 bytes are no UNSIGNED32.
	want(t, run(t, "", "sdo", "read", "5", "0x1017", "0", "-i", "virtual", "-t", "u32"), 6, "not a UNSIGNED32")
	// An abort: exit 11 with the abort code.
	r = run(t, "", "sdo", "read", "5", "0x5FFF", "0", "-i", "virtual")
	want(t, r, 11, "0x06020000")
	r = run(t, "", "sdo", "read", "5", "0x5FFF", "0", "-i", "virtual", "--format", "json")
	want(t, r, 11, `"abort": {`, `"code": "0x06020000"`, `"remote": true`, `"code": "sdo_abort"`)
	// No such node: timeout, exit 5.
	want(t, run(t, "", "sdo", "read", "9", "0x1000", "0", "-i", "virtual", "--timeout", "50ms"), 5, "timeout")
	// --output writes the raw data.
	out := filepath.Join(t.TempDir(), "name.bin")
	want(t, run(t, "", "sdo", "read", "5", "0x1008", "0", "-i", "virtual", "-o", out), 0, "bytes written to")
	if b, _ := os.ReadFile(out); string(b) != "OTF IO-401 8DI/8DO 2AI/2AO" {
		t.Fatalf("%q", b)
	}
}

func TestSDOWrite(t *testing.T) {
	fresh(t)
	n := ioNode(t, 5)
	want(t, run(t, "", "sdo", "write", "5", "0x1017", "0", "250", "--type", "u16", "-i", "virtual", "--verify"), 0, "written (2 bytes), verified")
	if v, _ := n.Dictionary().GetUint(0x1017, 0); v != 250 {
		t.Fatalf("0x1017 = %d", v)
	}
	want(t, run(t, "", "sdo", "write", "5", "0x6200", "1", "0xA5", "--eds", "testdata/io401.eds", "-i", "virtual", "--format", "json"), 0, `"value": 165`)
	// Read-only object: the device refuses.
	want(t, run(t, "", "sdo", "write", "5", "0x1000", "0", "1", "-t", "u32", "-i", "virtual"), 11, "0x06010002")
	// Usage and input errors send nothing.
	want(t, run(t, "", "sdo", "write", "5", "0x1017", "0", "1", "-i", "virtual"), 3, "--type or --eds")
	want(t, run(t, "", "sdo", "write", "5", "0x1017", "0", "x", "-t", "u16", "-i", "virtual"), 3, "invalid UNSIGNED16 value")
	want(t, run(t, "", "sdo", "write", "5", "0x1017", "0", "-t", "u16", "-i", "virtual"), 3, "the value or --file")
	want(t, run(t, "", "sdo", "write", "200", "0x1017", "0", "1", "-t", "u16", "-i", "virtual"), 3, "invalid node-ID")
	want(t, run(t, "", "sdo", "write", "5", "0x1017", "0", "1", "-t", "u16", "-i", "slcan:/dev/null"), 9, "--yes")
	// A file, block transfer: the device has no DOMAIN; write a string.
	in := filepath.Join(t.TempDir(), "name.txt")
	_ = os.WriteFile(in, []byte("x"), 0o644)
	want(t, run(t, "", "sdo", "write", "5", "0x1008", "0", "--file", in, "-i", "virtual"), 11, "0x06010002")
}

func TestSDOBlockDomain(t *testing.T) {
	fresh(t)
	n := builtinNode(t, 7, false)
	data := strings.Repeat("0123456789abcdef", 64)
	in := filepath.Join(t.TempDir(), "blob.bin")
	_ = os.WriteFile(in, []byte(data), 0o644)
	want(t, run(t, "", "sdo", "write", "7", "0x2003", "0", "--file", in, "--block", "-i", "virtual", "--format", "json"), 0, `"block": true`, `"size": 1024`)
	if b, _ := n.Dictionary().Get(0x2003, 0); string(b) != data {
		t.Fatalf("%d bytes", len(b))
	}
	out := filepath.Join(t.TempDir(), "back.bin")
	want(t, run(t, "", "sdo", "read", "7", "0x2003", "0", "--block", "-o", out, "-i", "virtual"), 0)
	if b, _ := os.ReadFile(out); string(b) != data {
		t.Fatalf("%d bytes back", len(b))
	}
	want(t, run(t, "", "sdo", "write", "7", "0x2002", "0", "hello world", "-t", "string", "-i", "virtual"), 0)
	want(t, run(t, "", "sdo", "read", "7", "0x2002", "0", "-i", "virtual"), 0, `"hello world"`)
}

func TestSDODump(t *testing.T) {
	fresh(t)
	ioNode(t, 5)
	r := run(t, "", "sdo", "dump", "5", "--eds", "testdata/io401.eds", "-i", "virtual", "--range", "1000-1018")
	want(t, r, 0, "0x1018:01", "Vendor-ID", "read, 0 failed")
	r = run(t, "", "sdo", "dump", "5", "--eds", "testdata/io401.eds", "-i", "virtual", "--format", "json")
	want(t, r, 0, `"command": "sdo dump"`)
	// A node lacking objects the EDS lists: findings.
	fresh(t)
	builtinNode(t, 5, false)
	want(t, run(t, "", "sdo", "dump", "5", "--eds", "testdata/io401.eds", "-i", "virtual", "--range", "6000-6FFF", "--format", "table"), 1, "error:")
	want(t, run(t, "", "sdo", "dump", "9", "--eds", "testdata/io401.eds", "-i", "virtual", "--timeout", "20ms"), 5, "does not answer")
	want(t, run(t, "", "sdo", "dump", "5", "-i", "virtual"), 2, "eds")
	want(t, run(t, "", "sdo", "dump", "5", "--eds", "testdata/io401.eds", "--range", "x"), 3, "--range")
}

var _ = canopen.NodeID(0)
