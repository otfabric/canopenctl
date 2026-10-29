// SPDX-License-Identifier: MIT

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otfabric/go-canopen"
	"github.com/otfabric/go-canopen/eds"
	"github.com/otfabric/go-canopen/od"
)

func TestEDSValidate(t *testing.T) {
	fresh(t)
	want(t, run(t, "", "eds", "validate", "testdata/io401.eds"), 0, "EDS valid")
	r := run(t, "", "eds", "validate", "testdata/io401.eds", "--format", "json")
	want(t, r, 0, `"valid": true`)
	golden(t, "eds-validate-io401.json", r.out)
	r = run(t, "", "eds", "validate", "testdata/broken.dcf")
	want(t, r, 1, "error: [", "DCF invalid")
	want(t, run(t, "", "eds", "validate", "testdata/broken.dcf", "--format", "table"), 1, "SEVERITY")
	r = run(t, "", "eds", "validate", "testdata/compact.eds", "--strict")
	if r.code != 0 && r.code != 1 {
		t.Fatal(r)
	}
	want(t, run(t, "", "eds", "validate", "testdata/missing.eds"), 3, "cannot read EDS")
	bad := filepath.Join(t.TempDir(), "bad.eds")
	_ = os.WriteFile(bad, []byte("key outside a section\n"), 0o644)
	want(t, run(t, "", "eds", "validate", bad, "--format", "json"), 3, `"code": "invalid_eds"`)
}

func TestEDSShow(t *testing.T) {
	fresh(t)
	r := run(t, "", "eds", "show", "testdata/io401.eds", "--range", "1800")
	want(t, r, 0, "IO-401", "0x1800:01", "$NODEID+0x180")
	golden(t, "eds-show-1800.txt", r.out)
	r = run(t, "", "eds", "show", "testdata/io401.eds", "--node-id", "5", "--range", "1800-1800", "--format", "json")
	want(t, r, 0, `"default": "0x00000185"`)
	golden(t, "eds-show-1800.json", r.out)
	want(t, run(t, "", "eds", "show", "testdata/drive402.eds", "--format", "table", "--range", "6040-607F"), 0, "0x607D:02", "PDO")
	want(t, run(t, "", "eds", "show", "testdata/node37.dcf", "--range", "1000"), 0, "IO-401")
	want(t, run(t, "", "eds", "show", "testdata/io401.eds", "--node-id", "200"), 3, "--node-id")
	want(t, run(t, "", "eds", "show", "testdata/io401.eds", "--range", "2000-1000"), 3, "--range")
}

func TestEDSConvert(t *testing.T) {
	fresh(t)
	dir := t.TempDir()
	dcf := filepath.Join(dir, "node5.dcf")
	want(t, run(t, "", "eds", "convert", "testdata/io401.eds", "--node-id", "5", "--node-name", "io", "--baud", "500", "-o", dcf), 0, "node5.dcf (dcf")
	f, err := eds.ParseFile(dcf)
	if err != nil || !f.IsDCF() || f.DeviceComissioning.NodeID != 5 || f.DeviceComissioning.NodeName != "io" {
		t.Fatal(err, f.DeviceComissioning)
	}
	if v := f.Object(0x1800).Sub(1).ParameterValue; v != "0x00000185" {
		t.Fatalf("1800sub1 ParameterValue %q", v)
	}
	want(t, run(t, "", "eds", "validate", dcf), 0)
	// Re-target the DCF, and back to an EDS.
	want(t, run(t, "", "eds", "convert", dcf, "--to", "dcf", "--node-id", "6", "--format", "json"), 0, "[DeviceComissioning]", "NodeID=6")
	back := filepath.Join(dir, "back.eds")
	want(t, run(t, "", "eds", "convert", dcf, "-o", back, "--format", "json"), 0, `"kind": "eds"`, `"sha256"`)
	g, err := eds.ParseFile(back)
	if err != nil || g.IsDCF() || g.Object(0x1800).Sub(1).HasParameterValue {
		t.Fatal(err)
	}
	// The DCF keeps its node-ID without --node-id.
	want(t, run(t, "", "eds", "convert", "testdata/node37.dcf", "--to", "dcf"), 0, "NodeID=37")
	want(t, run(t, "", "eds", "convert", "testdata/io401.eds", "--to", "dcf"), 3, "node-ID")
	want(t, run(t, "", "eds", "convert", "testdata/io401.eds", "--to", "xml"), 3, "--to")
	want(t, run(t, "", "eds", "convert", "testdata/io401.eds", "--node-id", "5", "--to", "dcf", "--baud", "5000"), 3, "--baud")
	want(t, run(t, "", "eds", "convert", "testdata/missing.eds", "--to", "eds"), 3, "cannot read")
	want(t, run(t, "", "eds", "convert", "testdata/io401.eds", "--to", "eds", "-o", filepath.Join(dir, "no", "such", "x.eds")), 10, "write")
}

func TestEDSGenerate(t *testing.T) {
	fresh(t)
	r := run(t, "", "eds", "generate", "testdata/io401.eds", "--package", "io401")
	want(t, r, 0, "package io401", "func NewDictionary(")
	out := filepath.Join(t.TempDir(), "gen", "io401.go")
	want(t, run(t, "", "eds", "generate", "testdata/io401.eds", "--package", "io401", "-o", out), 0, "io401.go (go,")
	if b, _ := os.ReadFile(out); !strings.Contains(string(b), "package io401") {
		t.Fatal("generated file")
	}
	want(t, run(t, "", "eds", "generate", "testdata/broken.dcf", "--package", "x"), 3, "eds validate")
	want(t, run(t, "", "eds", "generate", "testdata/io401.eds", "--package", "Bad-Name"), 3, "--package")
	want(t, run(t, "", "eds", "generate", "testdata/io401.eds"), 2, "package")
	want(t, run(t, "", "eds", "generate", "testdata/missing.eds", "--package", "x"), 3, "cannot read")
	for _, s := range []string{"", "_", "1a", "a-b"} {
		if validIdent(s) {
			t.Error(s)
		}
	}
}

func TestDCFApply(t *testing.T) {
	fresh(t)
	n := ioNode(t, 37)
	want(t, run(t, "", "dcf", "apply", "37", "testdata/node37.dcf", "--dry-run"), 0, "dry run:", "would be written to node 37")
	r := run(t, "", "dcf", "apply", "0", "testdata/node37.dcf", "-i", "virtual", "--format", "json")
	want(t, r, 0, `"node": 37`, `"dryRun": false`)
	f, _ := eds.ParseFile("testdata/node37.dcf")
	// Every configured, writable value of the DCF is now the node's.
	entries := 0
	for _, o := range f.Objects {
		for _, s := range o.Subs {
			if !s.HasParameterValue || !s.AccessType.Writable() || s.SubIndex == 0 {
				continue
			}
			want, err := canopen.ParseValue(s.DataType, s.ParameterValue, 37)
			if err != nil {
				continue
			}
			got, _ := n.Dictionary().Get(o.Index, s.SubIndex)
			if string(got) != string(want) {
				t.Errorf("%04X:%02X = %x, DCF %x", o.Index, s.SubIndex, got, want)
			}
			entries++
		}
	}
	if entries == 0 {
		t.Fatal("no entries checked")
	}
	want(t, run(t, "", "dcf", "apply", "37", "testdata/node37.dcf", "-i", "virtual", "--format", "table"), 0, "KEY")
	want(t, run(t, "", "dcf", "apply", "0", "testdata/io401.eds", "-i", "virtual"), 3, "NodeID")
	want(t, run(t, "", "dcf", "apply", "37", "testdata/node37.dcf", "-i", "slcan:/dev/null"), 9, "--yes")
	want(t, run(t, "", "dcf", "apply", "37", "testdata/missing.dcf", "-i", "virtual"), 3, "cannot read")
	want(t, run(t, "", "dcf", "apply", "38", "testdata/node37.dcf", "-i", "virtual", "--timeout", "20ms"), 5, "")
}

func TestDCFExport(t *testing.T) {
	fresh(t)
	n := ioNode(t, 5)
	_ = n.Dictionary().SetUint(od.IndexProducerHeartbeat, 0, 1234)
	out := filepath.Join(t.TempDir(), "node5.dcf")
	want(t, run(t, "", "dcf", "export", "5", "--eds", "testdata/io401.eds", "-o", out, "-i", "virtual", "--node-name", "io5"), 0, "values read, 0 failed")
	f, err := eds.ParseFile(out)
	if err != nil || !f.IsDCF() || f.DeviceComissioning.NodeID != 5 {
		t.Fatal(err)
	}
	if v := f.Object(0x1017).ParameterValue; v != "0x04D2" {
		t.Fatalf("1017 ParameterValue %q", v)
	}
	want(t, run(t, "", "eds", "validate", out), 0)
	// To stdout; a node lacking objects: findings.
	fresh(t)
	builtinNode(t, 5, false)
	r := run(t, "", "dcf", "export", "5", "--eds", "testdata/io401.eds", "-i", "virtual")
	want(t, r, 1, "[DeviceComissioning]", "EDS default kept")
	want(t, run(t, "", "dcf", "export", "9", "--eds", "testdata/io401.eds", "-i", "virtual", "--timeout", "20ms"), 5, "does not answer")
	want(t, run(t, "", "dcf", "export", "5", "-i", "virtual"), 2, "eds")
}
