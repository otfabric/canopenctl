// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otfabric/go-canopen"
	"github.com/otfabric/go-canopen/cdcf"
	"github.com/otfabric/go-canopen/lss"
	"github.com/otfabric/go-canopen/node"
	"github.com/otfabric/go-canopen/od"
	"github.com/otfabric/go-canopen/program"
	"github.com/otfabric/go-canopen/store"
)

// writeConcise writes a concise DCF to a temporary file.
func writeConcise(t *testing.T, entries ...cdcf.Entry) string {
	t.Helper()
	b, err := cdcf.Encode(entries)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "node.cdcf")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCDCF(t *testing.T) {
	fresh(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "node37.cdcf")
	want(t, run(t, "", "cdcf", "convert", "testdata/node37.dcf", "-o", out), 0, "(cdcf,", "entries")
	want(t, run(t, "", "cdcf", "convert", "testdata/node37.dcf", "-o", out, "--format", "json", "--node-id", "37"), 0, `"kind": "cdcf"`, `"nodeId": 37`)
	r := run(t, "", "cdcf", "convert", "testdata/node37.dcf")
	want(t, r, 0)
	b, _ := os.ReadFile(out)
	if r.out != string(b) {
		t.Fatal("stdout differs from the file")
	}
	entries, err := cdcf.Decode(b)
	if err != nil || len(entries) == 0 {
		t.Fatal(entries, err)
	}
	want(t, run(t, "", "cdcf", "show", out, "--eds", "testdata/node37.dcf"), 0, "entries,", "bytes as a concise DCF")
	want(t, run(t, "", "cdcf", "show", "testdata/node37.dcf", "--format", "json"), 0, `"format": "dcf"`, `"command": "cdcf show"`)
	want(t, run(t, "", "cdcf", "show", out, "--format", "table"), 0, "KEY")
	// Apply a concise DCF to a node: one entry.
	n := builtinNode(t, 5, false)
	path := writeConcise(t, cdcf.Entry{Index: simText, Data: []byte("configured")}, cdcf.Entry{Index: od.IndexProducerHeartbeat, Data: []byte{50, 0}})
	want(t, run(t, "", "cdcf", "apply", "5", path, "--dry-run", "--format", "json"), 0, `"dryRun": true`, `"key": "0x2002:00"`)
	want(t, run(t, "", "cdcf", "apply", "5", path, "-i", "virtual"), 0, "2 entries of")
	if v, _ := n.Dictionary().Get(simText, 0); string(v) != "configured" {
		t.Fatalf("%q", v)
	}
	// The first refusal stops it.
	bad := writeConcise(t, cdcf.Entry{Index: 0x1000, Data: []byte{1, 2, 3, 4}})
	want(t, run(t, "", "cdcf", "apply", "5", bad, "-i", "virtual"), 11, "0x06010002")
	// A DCF is converted.
	want(t, run(t, "", "cdcf", "apply", "5", "testdata/node37.dcf", "--dry-run"), 0, "would be written to node 5")
	garbage := filepath.Join(dir, "garbage.bin")
	_ = os.WriteFile(garbage, []byte{1, 2, 3}, 0o644)
	for _, c := range []struct {
		args []string
		code int
		msg  string
	}{
		{[]string{"cdcf", "convert", "testdata/missing.dcf"}, 3, "cannot read"},
		{[]string{"cdcf", "convert", "testdata/node37.dcf", "--node-id", "200"}, 3, "--node-id"},
		{[]string{"cdcf", "convert", "testdata/io401.eds"}, 3, "node-ID"},
		{[]string{"cdcf", "convert", "testdata/node37.dcf", "-o", filepath.Join(dir, "no", "such", "dir")}, 10, "write"},
		{[]string{"cdcf", "show", garbage}, 3, "neither a DCF nor a concise DCF"},
		{[]string{"cdcf", "show", filepath.Join(dir, "missing")}, 3, "cannot read"},
		{[]string{"cdcf", "show", "testdata/broken.dcf"}, 3, ""},
		{[]string{"cdcf", "show", "testdata/io401.eds"}, 3, "node-ID"},
		{[]string{"cdcf", "show", out, "--node-id", "-1"}, 3, "--node-id"},
		{[]string{"cdcf", "show", out, "--eds", "testdata/missing.eds"}, 3, "cannot read"},
		{[]string{"cdcf", "apply", "x", out}, 3, "node-ID"},
		{[]string{"cdcf", "apply", "5", out, "--timeout", "0s"}, 3, "--timeout"},
		{[]string{"cdcf", "apply", "5", out, "--brs"}, 3, "--brs"},
		{[]string{"cdcf", "apply", "5", garbage}, 3, "neither"},
		{[]string{"cdcf", "apply", "5", out, "--eds", "testdata/missing.eds"}, 3, "cannot read"},
		{[]string{"cdcf", "apply", "5", out, "-i", "slcan:/dev/null"}, 9, "--yes"},
		{[]string{"cdcf", "apply", "5", out, "-i", "slcan:/nonexistent", "--yes"}, 4, ""},
	} {
		want(t, run(t, "", c.args...), c.code, c.msg)
	}
}

// storeNode starts a built-in node with 0x1010/0x1011 and a memory
// store.
func storeNode(t *testing.T, id canopen.NodeID) (*node.Node, *store.Memory) {
	t.Helper()
	d, err := builtinDictionary(id, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddObjects(d); err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemory()
	return simNode(t, d, node.WithStore(mem)), mem
}

func TestStoreRestore(t *testing.T) {
	fresh(t)
	n, mem := storeNode(t, 5)
	builtinNode(t, 6, false)
	want(t, run(t, "", "sdo", "write", "5", "0x2002", "0", "saved", "-t", "string", "-i", "virtual"), 0)
	want(t, run(t, "", "store", "5", "--group", "manufacturer", "-i", "virtual"), 0, `saved its manufacturer parameters (0x1010:04 = "save")`)
	if mem.Saves() != 1 {
		t.Fatal("not saved")
	}
	want(t, run(t, "", "store", "5", "--format", "json", "-i", "virtual"), 0, `"action": "store"`, `"groupName": "all"`, `"key": "0x1010:01"`)
	want(t, run(t, "", "sdo", "write", "5", "0x2002", "0", "changed", "-t", "string", "-i", "virtual"), 0)
	want(t, run(t, "", "nmt", "reset-node", "5", "-i", "virtual", "--wait"), 0)
	if v, _ := n.Dictionary().Get(simText, 0); string(v) != "saved" {
		t.Fatalf("after reset: %q", v)
	}
	want(t, run(t, "", "restore", "5", "--reset", "-i", "virtual"), 0, "restores the defaults of its all parameters", "NMT reset node sent")
	eventually(t, "defaults", func() bool { v, _ := n.Dictionary().Get(simText, 0); return string(v) == "hello" })
	want(t, run(t, "", "restore", "5", "--group", "2", "--format", "json", "-i", "virtual"), 0, `"action": "restore"`, `"groupName": "communication"`)
	want(t, run(t, "", "restore", "5", "--group", "0x7F", "-i", "virtual"), 11, "")
	// A node without the objects refuses.
	want(t, run(t, "", "store", "6", "-i", "virtual"), 11, "0x06020000")
	for _, c := range []struct {
		args []string
		code int
		msg  string
	}{
		{[]string{"store", "x"}, 3, "node-ID"},
		{[]string{"store", "5", "--group", "x"}, 3, "invalid group"},
		{[]string{"store", "5", "--group", "200"}, 3, "invalid group"},
		{[]string{"store", "5", "--timeout", "0s"}, 3, "--timeout"},
		{[]string{"restore", "5", "--brs"}, 3, "--brs"},
		{[]string{"store", "5", "-i", "slcan:/dev/null"}, 9, "--yes"},
		{[]string{"store", "5", "-i", "slcan:/nonexistent", "--yes"}, 4, ""},
	} {
		args := c.args
		if !contains(args, "-i") {
			args = append(args, "-i", "virtual")
		}
		want(t, run(t, "", args...), c.code, c.msg)
	}
}

func TestProgram(t *testing.T) {
	fresh(t)
	d, _ := builtinDictionary(5, false)
	mem := &memProgram{}
	n := simNode(t, d, node.WithProgram(program.Config{Handler: mem}))
	builtinNode(t, 6, false)
	img := bytes.Repeat([]byte("canopenctl-firmware-"), 100)
	file := filepath.Join(t.TempDir(), "fw.bin")
	_ = os.WriteFile(file, img, 0o644)
	crc := fmt.Sprintf("0x%08X", crc32.ChecksumIEEE(img))
	want(t, run(t, "", "program", "download", "5", file, "--crc32", "--poll", "5ms", "-i", "virtual"), 0, "2000 bytes of", "started", crc)
	if !bytes.Equal(mem.running, img) || n.Program().State(1) != program.Started {
		t.Fatal("program not running")
	}
	want(t, run(t, "", "program", "status", "5", "-i", "virtual"), 0, "program 1: started, identification "+crc, "flash status 0x00000000 (ok)")
	want(t, run(t, "", "program", "status", "5", "-i", "virtual", "--format", "json"), 0, `"stateValue": 1`, `"flashText": "ok"`)
	want(t, run(t, "", "program", "control", "5", "stop", "-i", "virtual"), 0, "program 1: stop sent")
	want(t, run(t, "", "program", "control", "5", "clear", "-i", "virtual", "--format", "json"), 0, `"action": "clear"`)
	want(t, run(t, "", "program", "status", "5", "-i", "virtual"), 0, "cleared")
	// A wrong identification: exit 12.
	r := run(t, "", "program", "download", "5", file, "--identification", "0x1", "--poll", "5ms", "-i", "virtual", "--format", "json")
	want(t, r, 12, `"code": "program_failed"`)
	want(t, run(t, "", "program", "download", "5", file, "--poll", "5ms", "-i", "virtual", "--format", "json"), 0, `"command": "program download"`)
	// In operational the device refuses program control.
	want(t, run(t, "", "nmt", "start", "5", "-i", "virtual"), 0)
	eventually(t, "operational", func() bool { return n.State() == canopen.StateOperational })
	want(t, run(t, "", "program", "control", "5", "start", "-i", "virtual"), 11, "")
	// A node without the objects.
	want(t, run(t, "", "program", "status", "6", "-i", "virtual"), 11, "0x06020000")
	empty := filepath.Join(t.TempDir(), "empty.bin")
	_ = os.WriteFile(empty, nil, 0o644)
	for _, c := range []struct {
		args []string
		code int
		msg  string
	}{
		{[]string{"download", "x", file}, 3, "node-ID"},
		{[]string{"download", "5", file, "--program", "0"}, 3, "--program"},
		{[]string{"download", "5", file, "--crc32", "--identification", "1"}, 3, "not both"},
		{[]string{"download", "5", file, "--poll", "0s"}, 3, "--poll"},
		{[]string{"download", "5", file, "--identification", "x"}, 3, "invalid value"},
		{[]string{"download", "5", "missing.bin"}, 3, "cannot read"},
		{[]string{"download", "5", empty}, 3, "is empty"},
		{[]string{"download", "5", file, "-i", "slcan:/dev/null"}, 9, "--yes"},
		{[]string{"download", "5", file, "-i", "slcan:/nonexistent", "--yes"}, 4, ""},
		{[]string{"control", "x", "stop"}, 3, "node-ID"},
		{[]string{"control", "5", "pause"}, 3, "unknown program command"},
		{[]string{"control", "5", "stop", "--brs"}, 3, "--brs"},
		{[]string{"control", "5", "stop", "-i", "slcan:/dev/null"}, 9, "--yes"},
		{[]string{"control", "5", "stop", "-i", "slcan:/nonexistent", "--yes"}, 4, ""},
		{[]string{"status", "x"}, 3, "node-ID"},
		{[]string{"status", "5", "--program", "300"}, 3, "--program"},
		{[]string{"status", "5", "-i", "slcan:/nonexistent"}, 4, ""},
		{[]string{"status", "9", "--timeout", "20ms"}, 5, ""},
	} {
		args := append([]string{"program"}, c.args...)
		if !contains(args, "-i") {
			args = append(args, "-i", "virtual")
		}
		want(t, run(t, "", args...), c.code, c.msg)
	}
}

func TestBoot(t *testing.T) {
	fresh(t)
	n5 := builtinNode(t, 5, false)
	n6 := builtinNode(t, 6, false)
	cfg := writeConcise(t, cdcf.Entry{Index: simText, Data: []byte("booted")})
	r := run(t, "", "boot", "--slave", "5="+cfg, "--slave", "6", "--slave", "7", "--optional", "7", "--expect", "5=F7A:C0C7:10000:5",
		"--boot-time", "3s", "-i", "virtual")
	want(t, r, 1, "node   5  mandatory  booted    booted, 1 entries of", "node   7  optional   failed    not booted (no boot-up)",
		"2 of 3 slaves booted")
	if v, _ := n5.Dictionary().Get(simText, 0); string(v) != "booted" {
		t.Fatalf("%q", v)
	}
	eventually(t, "started", func() bool {
		return n5.State() == canopen.StateOperational && n6.State() == canopen.StateOperational
	})
	// Every slave there: exit 0; --no-start leaves them pre-operational.
	r = run(t, "", "boot", "--slave", "5", "--slave", "6", "--no-start", "--format", "json", "-i", "virtual")
	want(t, r, 0, `"command": "boot"`, `"booted": 2`, `"started": false`)
	validateOutput(t, r)
	eventually(t, "pre-operational", func() bool { return n5.State() == canopen.StatePreOperational })
	want(t, run(t, "", "boot", "--slave", "5", "--no-start", "--format", "table", "-i", "virtual"), 0, "MANDATORY")
	// A wrong identity fails a mandatory slave: status D.
	r = run(t, "", "boot", "--slave", "5", "--expect", "5=BAD:0:0:0", "--boot-time", "2s", "-i", "virtual")
	want(t, r, 1, "(D)", "vendor-ID")
	for _, c := range []struct {
		args []string
		code int
		msg  string
	}{
		{[]string{}, 3, "at least one --slave"},
		{[]string{"--slave", "x"}, 3, "node-ID"},
		{[]string{"--slave", "x=y"}, 3, "node-ID"},
		{[]string{"--slave", "5="}, 3, "NODE=VALUE"},
		{[]string{"--slave", "127"}, 3, "master's node-ID"},
		{[]string{"--slave", "5", "--slave", "5"}, 3, "twice"},
		{[]string{"--slave", "5=missing.cdcf"}, 3, "cannot read"},
		{[]string{"--slave", "5", "--optional", "6"}, 3, "not a --slave"},
		{[]string{"--slave", "5", "--optional", "x"}, 3, "node"},
		{[]string{"--slave", "5", "--expect", "6=1:2:3:4"}, 3, "not a --slave"},
		{[]string{"--slave", "5", "--expect", "5=1:2"}, 3, "LSS address"},
		{[]string{"--slave", "5", "--expect", "x"}, 3, "NODE=VALUE"},
		{[]string{"--slave", "5", "--lss"}, 3, "needs a non-zero --expect"},
		{[]string{"--slave", "5", "--master-id", "0"}, 3, "--master-id"},
		{[]string{"--slave", "5", "--max-time", "0s"}, 3, "--max-time"},
		{[]string{"--slave", "5", "--timeout", "0s"}, 3, "--timeout"},
		{[]string{"--slave", "5", "--brs"}, 3, "--brs"},
		{[]string{"--slave", "5", "-i", "slcan:/dev/null"}, 9, "--yes"},
		{[]string{"--slave", "5", "-i", "slcan:/nonexistent", "--yes"}, 4, ""},
	} {
		args := append([]string{"boot"}, c.args...)
		if !contains(args, "-i") {
			args = append(args, "-i", "virtual")
		}
		want(t, run(t, "", args...), c.code, c.msg)
	}
}

func TestBootLSS(t *testing.T) {
	fresh(t)
	a := lssNode(t, 0, 0x1001)
	b := lssNode(t, 0, 0x1002)
	cfg := writeConcise(t, cdcf.Entry{Index: simText, Data: []byte("via lss")})
	r := run(t, "", "boot", "--lss", "--lss-window", "10ms", "--slave", "5="+cfg, "--expect", "5=F7A:C0C7:10000:1001",
		"--slave", "6", "--expect", "6=F7A:C0C7:10000:1002", "-i", "virtual")
	want(t, r, 0, "node-ID given by LSS to "+lssAddr(0x1001), "2 of 2 slaves booted")
	if a.NodeID() != 5 || b.NodeID() != 6 || !strings.Contains(r.out, lssAddr(0x1002)) {
		t.Fatal(a.NodeID(), b.NodeID())
	}
	if v, _ := a.Dictionary().Get(simText, 0); string(v) != "via lss" {
		t.Fatalf("%q", v)
	}
	_ = lss.Unconfigured
}
