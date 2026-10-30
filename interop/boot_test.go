// SPDX-License-Identifier: MIT

//go:build interop

package interop

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// otherProductEDS is the shared device with another product code: a
// device of interop-device.eds does not match its identity.
const otherProductEDS = "/fixtures/eds/interop-device-other-product.eds"

// edsValue is the DefaultValue of a section of an EDS file.
func edsValue(t *testing.T, path, section string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	in := false
	s := bufio.NewScanner(f)
	for s.Scan() {
		l := strings.TrimSpace(s.Text())
		if strings.HasPrefix(l, "[") {
			in = strings.EqualFold(l, "["+section+"]")
		} else if k, v, ok := strings.Cut(l, "="); in && ok && strings.EqualFold(k, "DefaultValue") {
			return v
		}
	}
	t.Fatalf("%s: no DefaultValue in [%s]", path, section)
	return ""
}

// identity is the LSS address (vendor:product:revision:0, serial not
// checked) of an EDS file's 0x1018 in boot --expect's notation.
func identity(t *testing.T, path string) string {
	t.Helper()
	var p []string
	for _, sec := range []string{"1018sub1", "1018sub2", "1018sub3"} {
		v, err := strconv.ParseUint(edsValue(t, path, sec), 0, 32)
		if err != nil {
			t.Fatalf("%s [%s]: %v", path, sec, err)
		}
		p = append(p, fmt.Sprintf("%X", v))
	}
	return strings.Join(p, ":") + ":0"
}

// bootConfig writes a concise DCF (CiA 302-3) for node id: 0x2001:00 =
// 0x4400+id (UNSIGNED16) and the heartbeat producer time 0x1017:00 = 200
// ms, encoded here independently of go-canopen.
func bootConfig(t *testing.T, dir string, id int) string {
	t.Helper()
	b := binary.LittleEndian.AppendUint32(nil, 2)
	for _, e := range []struct {
		index uint16
		v     uint16
	}{{0x2001, uint16(0x4400 + id)}, {0x1017, 200}} {
		b = binary.LittleEndian.AppendUint16(b, e.index)
		b = append(b, 0)
		b = binary.LittleEndian.AppendUint32(b, 2)
		b = binary.LittleEndian.AppendUint16(b, e.v)
	}
	p := filepath.Join(dir, fmt.Sprintf("node%d.cdcf", id))
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// booted checks a node booted by a CiA 302 master with bootConfig: the
// concise DCF's values by SDO and the state operational by heartbeat.
func booted(t *testing.T, ifc string, id int) {
	t.Helper()
	r := ctl(t, "sdo", "read", fmt.Sprint(id), "0x2001", "0", "-t", "u16", "-i", ifc)
	want(t, r, 0, fmt.Sprintf("(0x%04X)", 0x4400+id))
	want(t, ctl(t, "sdo", "read", fmt.Sprint(id), "0x1017", "0", "-t", "u16", "-i", ifc), 0, "200 (0x00C8)")
	want(t, ctl(t, "heartbeat", "watch", "--node", fmt.Sprint(id), "--count", "1", "--duration", "5s", "-i", ifc, "--format", "json"),
		0, `"state":"operational"`)
}

// slaveOf is the slave id of boot's JSON result.
func slaveOf(t *testing.T, r result, id int) map[string]any {
	t.Helper()
	list, _ := resultOf(t, r)["slaves"].([]any)
	for _, x := range list {
		if s := x.(map[string]any); s["node"] == float64(id) {
			return s
		}
	}
	t.Fatalf("no slave %d in the result: %s", id, r.out)
	return nil
}

// TestBoot: canopenctl boot (go-canopen's CiA 302 manager) boots each
// stack's device: identity check, concise DCF, start; an identity that
// differs (error status M, exit 1); and the LSS step (boot --lss) for the
// stacks' unconfigured LSS slaves.
func TestBoot(t *testing.T) {
	ifc := needVCAN(t)
	for _, a := range adapters {
		t.Run(a, func(t *testing.T) {
			c := needImage(t, a)
			dir := t.TempDir()
			fx := fixture(t, a, "eds/interop-device.eds", dir)
			other := fixture(t, a, "eds/interop-device-other-product.eds", dir)
			device := func(t *testing.T, id int) {
				t.Helper()
				startContainer(t, a, "device", "--interface", ifc, "--node-id", fmt.Sprint(id), "--eds", deviceEDS,
					"--timeout-ms", "900000").await(t, `"type":"ready"`, 90*time.Second)
			}
			boot := func(t *testing.T, args ...string) result {
				return ctl(t, append([]string{"boot", "-i", ifc, "--boot-time", "20s", "--max-time", "90s", "--format", "json"}, args...)...)
			}
			scenario(t, a, "boot-slave", func(t *testing.T) {
				device(t, 21)
				r := boot(t, "--slave", "21="+bootConfig(t, dir, 21), "--expect", "21="+identity(t, fx))
				want(t, r, 0, `"booted": 1`, `"started": true`)
				if s := slaveOf(t, r, 21); s["booted"] != true || s["entries"] != 2.0 {
					t.Fatalf("slave 21: %v", s)
				}
				booted(t, ifc, 21)
			})
			scenario(t, a, "boot-identity-mismatch", func(t *testing.T) {
				device(t, 24)
				exp := identity(t, other)
				r := boot(t, "--slave", "24", "--expect", "24="+exp)
				want(t, r, 1, `"failed": 1`)
				if s := slaveOf(t, r, 24); s["booted"] != false || s["code"] != "M" {
					t.Fatalf("slave 24 (expected %s): %v, want error status M", exp, s)
				}
				if res := resultOf(t, r); res["started"] != false {
					t.Fatalf("started: %v", res["started"])
				}
			})
			scenario(t, a, "boot-lss", func(t *testing.T) {
				if !c.Features["lssSlave"] {
					capSkip(t, "%s's device has no LSS slave (lssSlave false)", a)
				}
				serial := lssSerial(a, 2)
				lssDevice(t, a, ifc, serial)
				exp := strings.TrimSuffix(identity(t, fx), ":0") + fmt.Sprintf(":%X", serial)
				r := boot(t, "--lss", "--slave", "22="+bootConfig(t, dir, 22), "--expect", "22="+exp)
				want(t, r, 0, `"booted": 1`)
				s := slaveOf(t, r, 22)
				l, _ := s["lss"].(map[string]any)
				if s["booted"] != true || l["address"] != lssAddr(serial) {
					t.Fatalf("slave 22: %v, want booted with the node-ID given by LSS to %s", s, lssAddr(serial))
				}
				booted(t, ifc, 22)
			})
		})
	}
}

// TestBootNetwork: canopenctl boot of one network with a device of every
// stack (and a canopenctl simulate device): every slave booted, configured
// and started.
func TestBootNetwork(t *testing.T) {
	ifc := needVCAN(t)
	name := strings.Join(adapters, "+")
	dir := t.TempDir()
	for _, a := range adapters {
		needImage(t, a)
	}
	fx := fixture(t, adapters[0], "eds/interop-device.eds", dir)
	scenario(t, name, "boot-network", func(t *testing.T) {
		args := []string{"boot", "-i", ifc, "--boot-time", "20s", "--max-time", "90s", "--format", "json"}
		ids := []int{}
		for i, a := range adapters {
			id := 21 + i
			startContainer(t, a, "device", "--interface", ifc, "--node-id", fmt.Sprint(id), "--eds", deviceEDS,
				"--timeout-ms", "900000").await(t, `"type":"ready"`, 90*time.Second)
			args = append(args, "--slave", fmt.Sprintf("%d=%s", id, bootConfig(t, dir, id)), "--expect", fmt.Sprintf("%d=%s", id, identity(t, fx)))
			ids = append(ids, id)
		}
		sim := startCtl(t, "simulate", "-i", ifc, "--eds", fx, "--node-id", "25", "--format", "json")
		sim.await(t, `"type":"ready"`, 10*time.Second)
		args = append(args, "--slave", "25="+bootConfig(t, dir, 25), "--expect", "25="+identity(t, fx))
		ids = append(ids, 25)
		r := ctl(t, args...)
		want(t, r, 0, fmt.Sprintf(`"booted": %d`, len(ids)), `"started": true`)
		for _, id := range ids {
			booted(t, ifc, id)
		}
	})
}

// TestStackBoot: each stack's CiA 302 boot master (boot) boots canopenctl
// simulate (the shared EDS): the boot slave process with a concise DCF of
// the stack's configuration tool, and an identity that differs (error
// status M).
func TestStackBoot(t *testing.T) {
	ifc := needVCAN(t)
	for _, a := range adapters {
		t.Run(a, func(t *testing.T) {
			c := needImage(t, a)
			dir := t.TempDir()
			fx := fixture(t, a, "eds/interop-device.eds", dir)
			need := func(t *testing.T) {
				if !c.op("boot") || !c.Features["boot"] {
					capSkip(t, "%s has no CiA 302 boot master (boot)", a)
				}
			}
			bootEvent := func(t *testing.T, out string, id int) map[string]any {
				var last map[string]any
				for _, d := range docs(t, out) {
					if d["type"] == "boot" && d["node"] == float64(id) {
						last = d
					}
				}
				return last
			}
			scenario(t, a, "stack-boot-slave", func(t *testing.T) {
				need(t)
				sim := startCtl(t, "simulate", "-i", ifc, "--eds", fx, "--node-id", "21", "--format", "json")
				sim.await(t, `"type":"ready"`, 10*time.Second)
				code, out := adapterRun(t, a, "boot", "--interface", ifc, "--timeout-ms", "20000",
					"--slave", "21:"+deviceEDS+":mandatory", "--sdo", "21:0x2001:00=0x4415", "--sdo", "21:0x1017:00=200")
				e := bootEvent(t, out, 21)
				if code != 0 || e == nil || e["ok"] != true || e["errorStatus"] != nil {
					t.Fatalf("boot: exit %d, event %v\n%s", code, e, out)
				}
				if !sim.seen(5*time.Second, `"type":"state"`, `"to":"operational"`) {
					t.Fatalf("simulate not started: %s", sim.stdout())
				}
				booted(t, ifc, 21)
			})
			scenario(t, a, "stack-boot-identity-mismatch", func(t *testing.T) {
				need(t)
				sim := startCtl(t, "simulate", "-i", ifc, "--eds", fx, "--node-id", "24", "--format", "json")
				sim.await(t, `"type":"ready"`, 10*time.Second)
				code, out := adapterRun(t, a, "boot", "--interface", ifc, "--timeout-ms", "20000", "--slave", "24:"+otherProductEDS+":mandatory")
				e := bootEvent(t, out, 24)
				if code != 5 || e == nil || e["errorStatus"] != "M" {
					t.Fatalf("boot: exit %d, event %v; want exit 5 and error status M\n%s", code, e, out)
				}
				if sim.seen(time.Second, `"to":"operational"`) {
					t.Fatalf("simulate was started although its identity differs: %s", sim.stdout())
				}
			})
		})
	}
}
