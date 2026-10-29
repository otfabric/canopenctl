// SPDX-License-Identifier: MIT

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSimulateLSS(t *testing.T) {
	fresh(t)
	storePath := filepath.Join(t.TempDir(), "node.cdcf")
	r := simulateWith(t, []string{"--lss", "--serial", "0x1234", "--store", storePath, "--program", "--format", "json"}, func() {
		want(t, run(t, "", "lss", "identify", "-i", "virtual"), 0, "a device without node-ID answered")
		want(t, run(t, "", "lss", "configure", "--address", lssAddr(0x1234), "--node-id", "9", "--store", "-i", "virtual"), 0, "node-ID 9, stored")
		eventually(t, "boot", func() bool {
			return run(t, "", "sdo", "read", "9", "0x1018", "4", "-i", "virtual", "--timeout", "50ms").code == 0
		})
		want(t, run(t, "", "store", "9", "-i", "virtual"), 0, "saved")
		want(t, run(t, "", "program", "status", "9", "-i", "virtual"), 0, "started")
	})
	want(t, r, 0, `"type":"ready"`, `"node":255`, `"lss":true`, `"program":true`)
	validateOutput(t, r)
	if b, err := os.ReadFile(storePath); err != nil || len(b) == 0 {
		t.Fatal("store file", err)
	}
	// The stored node-ID survives the restart.
	r = simulateWith(t, []string{"--lss", "--serial", "0x1234", "--store", storePath, "--duration", "5s"}, func() {
		want(t, run(t, "", "sdo", "read", "9", "0x1018", "4", "-i", "virtual"), 0, "4660")
	})
	want(t, r, 0, "node 9 running")
	// An EDS with --lss and --serial; text output names the unconfigured node.
	r = simulateWith(t, []string{"--lss", "--eds", "testdata/io401.eds", "--serial", "77"}, func() {
		want(t, run(t, "", "lss", "fastscan", "-i", "virtual", "--window", "10ms", "--vendor", "0xF7A"), 0, ":0000004D")
	})
	want(t, r, 0, "unconfigured LSS node running")
	// CANopen FD.
	r = simulateWith(t, []string{"--node-id", "5", "--fd"}, func() {
		want(t, run(t, "", "sdo", "read", "5", "0x1008", "0", "-i", "virtual", "--fd"), 0, "simulated node")
	})
	want(t, r, 0, "node 5 running")
	for _, c := range []struct {
		args []string
		msg  string
	}{
		{[]string{"--lss", "--serial", "x"}, "--serial"},
		{[]string{"--node-id", "5", "--eds", "testdata/compact.eds", "--serial", "1"}, "0x1018:04"},
		{[]string{"--lss", "--eds", "testdata/compact.eds"}, "identity"},
	} {
		r := run(t, "", append([]string{"simulate", "-i", "virtual", "--duration", "10ms"}, c.args...)...)
		if r.code == 0 || !strings.Contains(r.out+r.errOut, c.msg) {
			t.Fatalf("%v: exit %d %s %s", c.args, r.code, r.out, r.errOut)
		}
	}
}
