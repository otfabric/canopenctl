// SPDX-License-Identifier: MIT

//go:build interop

package interop

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// cdcfConfig are the configuration values (index, sub-index 0, integer
// value) of the shared device that the scenarios write: dcfgen takes them
// as --sdo, canopenctl as the ParameterValues of a DCF.
var cdcfConfig = []struct {
	index string
	value string
}{
	{"1017", "300"}, {"2000", "127"}, {"2001", "17493"}, {"2003", "-2"}, {"2006", "2"},
	{"2007", "1234605616436508552"}, {"200A", "150"},
}

// cdcfObjects are the ParameterValues of a DCF in which every sub-object
// of 0x2000..0x2004 has one: Lely's objects generator over that range and
// canopenctl's cdcf convert must give the same concise DCF.
var cdcfObjects = []struct {
	index string
	value string
}{
	{"2000", "0x5A"}, {"2001", "0x4455"}, {"2002", "305419896"}, {"2003", "-7"}, {"2004", "configured by canopenctl"},
}

// writeDCF writes a copy of the EDS with ParameterValue lines in the
// sections of values (sub-index 0 of VAR objects), readable by the
// containers' user.
func writeDCF(t *testing.T, eds, dir, name string, values []struct{ index, value string }) string {
	t.Helper()
	b, err := os.ReadFile(eds)
	if err != nil {
		t.Fatal(err)
	}
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	for _, v := range values {
		head := "\n[" + v.index + "]\n"
		i := strings.Index(s, head)
		if i < 0 {
			t.Fatalf("%s has no [%s]", eds, v.index)
		}
		i += len(head)
		s = s[:i] + "ParameterValue=" + v.value + "\n" + s[i:]
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// workDir is a directory the containers can read (mounted at /work).
func workDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// convert runs canopenctl cdcf convert and returns the concise DCF.
func convert(t *testing.T, dcf string) []byte {
	t.Helper()
	out := strings.TrimSuffix(dcf, filepath.Ext(dcf)) + ".cdcf"
	want(t, ctl(t, "cdcf", "convert", dcf, "--node-id", "5", "-o", out), 0)
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestConciseDCF: canopenctl cdcf convert against each stack's concise DCF
// generator (cdcf), byte for byte, for the same DCF: the configuration
// entries of the stack's configuration tool (dcfgen) and the dictionary
// dump of an index range (objects); canopenctl cdcf show decodes the
// stack's file.
func TestConciseDCF(t *testing.T) {
	for _, a := range adapters {
		t.Run(a, func(t *testing.T) {
			c := needImage(t, a)
			dir := workDir(t)
			fx := fixture(t, a, "eds/interop-device.eds", dir)
			need := func(t *testing.T, feature string) {
				if !c.op("cdcf") || !c.Features[feature] {
					capSkip(t, "%s has no concise DCF generator (cdcf, %s false)", a, feature)
				}
			}
			stack := func(t *testing.T, args ...string) []byte {
				t.Helper()
				code, out := adapterRunIn(t, a, dir, 2*time.Minute, append([]string{"cdcf", "--node-id", "5"}, args...)...)
				d := lastDoc(t, out)
				data, _ := d["data"].(string)
				b, err := hex.DecodeString(data)
				if code != 0 || err != nil {
					t.Fatalf("cdcf: exit %d: %s", code, out)
				}
				return b
			}
			scenario(t, a, "cdcf-convert-dcfgen", func(t *testing.T) {
				need(t, "cdcfDcfgen")
				var vals []struct{ index, value string }
				args := []string{"--dcf", "/work/config.dcf", "--generator", "dcfgen"}
				for _, v := range cdcfConfig {
					vals = append(vals, struct{ index, value string }{v.index, v.value})
					args = append(args, "--sdo", "0x"+v.index+":00="+v.value)
				}
				dcf := writeDCF(t, fx, dir, "config.dcf", vals)
				ours, theirs := convert(t, dcf), stack(t, args...)
				if !bytes.Equal(ours, theirs) {
					t.Fatalf("canopenctl %x\n%s %x", ours, a, theirs)
				}
				_ = os.WriteFile(filepath.Join(dir, "theirs.cdcf"), theirs, 0o644)
				want(t, ctl(t, "cdcf", "show", filepath.Join(dir, "theirs.cdcf"), "--eds", fx), 0,
					fmt.Sprintf("%d entries, %d bytes", len(cdcfConfig), len(theirs)), "0x2007:00 = 8877665544332211")
			})
			scenario(t, a, "cdcf-convert-objects", func(t *testing.T) {
				need(t, "cdcfObjects")
				var vals []struct{ index, value string }
				for _, v := range cdcfObjects {
					vals = append(vals, struct{ index, value string }{v.index, v.value})
				}
				dcf := writeDCF(t, fx, dir, "objects.dcf", vals)
				ours := convert(t, dcf)
				theirs := stack(t, "--dcf", "/work/objects.dcf", "--generator", "objects", "--range", "0x2000-0x2004")
				if !bytes.Equal(ours, theirs) {
					t.Fatalf("canopenctl %x\n%s %x", ours, a, theirs)
				}
			})
		})
	}
}

// probe reports whether a node has an object (an SDO upload of it does
// not end with abort 0x06020000) and what was observed.
func probe(t *testing.T, ifc string, node int, index, sub string) (bool, string) {
	t.Helper()
	r := ctl(t, "sdo", "read", fmt.Sprint(node), index, sub, "-i", ifc)
	switch {
	case r.code == 0:
		return true, ""
	case r.code == 11 && strings.Contains(r.err, "0x06020000"):
		return false, fmt.Sprintf("sdo read %s:%s aborted with 0x06020000 (object does not exist)", index, sub)
	}
	t.Fatalf("sdo read %s:%s: exit %d\n%s%s", index, sub, r.code, r.out, r.err)
	return false, ""
}

// TestStackDeviceCiA302: canopenctl's CiA 302 commands against each
// stack's device (node 5, interop-device.eds): cdcf apply of a converted
// DCF, read back by SDO; store and restore (0x1010/0x1011) and program
// (0x1F50/0x1F51) where the device has the objects; CANopen FD.
func TestStackDeviceCiA302(t *testing.T) {
	ifc := needVCAN(t)
	for _, a := range adapters {
		t.Run(a, func(t *testing.T) {
			needImage(t, a)
			dir := t.TempDir()
			fx := fixture(t, a, "eds/interop-device.eds", dir)
			dev := startContainer(t, a, "device", "--interface", ifc, "--node-id", "5", "--eds", deviceEDS, "--timeout-ms", "900000",
				"--dump", "0x2001:00", "--dump", "0x2007:00")
			dev.await(t, `"type":"ready"`, 60*time.Second)
			scenario(t, a, "cdcf-apply", func(t *testing.T) {
				var vals []struct{ index, value string }
				for _, v := range cdcfConfig {
					vals = append(vals, struct{ index, value string }{v.index, v.value})
				}
				dcf := writeDCF(t, fx, dir, "config.dcf", vals)
				convert(t, dcf)
				file := strings.TrimSuffix(dcf, ".dcf") + ".cdcf"
				r := ctl(t, "cdcf", "apply", "5", file, "--eds", fx, "-i", ifc, "--format", "json")
				want(t, r, 0)
				entries, _ := resultOf(t, r)["entries"].([]any)
				if len(entries) != len(cdcfConfig) {
					t.Fatalf("%d entries applied, want %d: %s", len(entries), len(cdcfConfig), r.out)
				}
				for _, x := range entries {
					e := x.(map[string]any)
					key := e["key"].(string)
					idx, sub, _ := strings.Cut(key, ":")
					rr := ctl(t, "sdo", "read", "5", idx, sub, "--eds", fx, "-i", ifc, "--format", "json")
					want(t, rr, 0)
					if got := resultOf(t, rr)["data"]; got != e["data"] {
						t.Fatalf("%s reads %v after cdcf apply, want %v", key, got, e["data"])
					}
				}
				res := dev.stop(t)
				vs, _ := res["values"].(map[string]any)
				if vs["0x2001:00"] != "5544" || vs["0x2007:00"] != "8877665544332211" {
					t.Fatalf("device dump %v, want 0x2001:00 5544 and 0x2007:00 8877665544332211", vs)
				}
			})
		})
	}
}

// TestStackDeviceStoreProgram: store/restore and program against each
// stack's device where it has the objects (0x1010/0x1011, 0x1F50/0x1F51),
// and the CANopen FD commands.
func TestStackDeviceStoreProgram(t *testing.T) {
	ifc := needVCAN(t)
	for _, a := range adapters {
		t.Run(a, func(t *testing.T) {
			c := needImage(t, a)
			dev := startContainer(t, a, "device", "--interface", ifc, "--node-id", "5", "--eds", deviceEDS, "--timeout-ms", "900000")
			dev.await(t, `"type":"ready"`, 60*time.Second)
			scenario(t, a, "store", func(t *testing.T) {
				for _, sub := range []string{"0", "1"} {
					if ok, why := probe(t, ifc, 5, "0x1010", sub); !ok {
						capSkip(t, "%s's device (interop-device.eds, the contract's device) has no store parameters 0x1010: %s", a, why)
					}
				}
				want(t, ctl(t, "store", "5", "-i", ifc), 0, "saved its all parameters")
			})
			scenario(t, a, "restore", func(t *testing.T) {
				if ok, why := probe(t, ifc, 5, "0x1011", "1"); !ok {
					capSkip(t, "%s's device (interop-device.eds, the contract's device) has no restore default parameters 0x1011: %s", a, why)
				}
				want(t, ctl(t, "restore", "5", "--reset", "-i", ifc), 0, "NMT reset node sent")
			})
			scenario(t, a, "program", func(t *testing.T) {
				for _, idx := range []string{"0x1F50", "0x1F51"} {
					if ok, why := probe(t, ifc, 5, idx, "1"); !ok {
						capSkip(t, "%s's device (interop-device.eds, the contract's device) has no program download objects: %s", a, why)
					}
				}
				want(t, ctl(t, "program", "status", "5", "-i", ifc), 0)
			})
			scenario(t, a, "fd", func(t *testing.T) {
				for k, v := range c.Features {
					l := strings.ToLower(k)
					// No contract 1.1 feature is CANopen FD; a later one would be
					// named so (fd..., ...usdo..., canopenFd...).
					if v && (strings.HasPrefix(l, "fd") || strings.Contains(l, "usdo") || strings.Contains(l, "canopenfd")) {
						t.Fatalf("%s declares %s: add the CANopen FD scenarios", a, k)
					}
				}
				capSkip(t, "%s declares no CANopen FD (CiA 1301) feature: the pinned stack has no USDO and no FD PDOs (canopen-interop LIMITATIONS.md); --fd needs a CAN FD peer", a)
			})
		})
	}
}
