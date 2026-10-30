// SPDX-License-Identifier: MIT

//go:build interop

package interop

import (
	"fmt"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// The LSS address of the shared test device (0x1018 of interop-device.eds)
// except the serial number, which each scenario chooses (device --serial,
// simulate --serial): every device of the suite has its own address.
const (
	lssVendor   uint32 = 0
	lssProduct  uint32 = 0x434F4931
	lssRevision uint32 = 0x00010000
)

// lssAddr is the address in canopenctl's notation.
func lssAddr(serial uint32) string {
	return fmt.Sprintf("%08X:%08X:%08X:%08X", lssVendor, lssProduct, lssRevision, serial)
}

// lssKnown are the known parts of the address: lss fastscan and
// assign-all verify them instead of scanning them.
var lssKnown = []string{"--vendor", fmt.Sprint(lssVendor), "--product", fmt.Sprintf("0x%08X", lssProduct),
	"--revision", fmt.Sprintf("0x%08X", lssRevision)}

// lssSerial is the first serial number of an adapter's LSS scenarios
// (dir 0: canopenctl's master, 1: the stack's master, 2: boot --lss).
func lssSerial(a string, dir int) uint32 {
	for i, x := range adapters {
		if x == a {
			return uint32(0x5100 + 0x100*dir + 0x10*i)
		}
	}
	return 0x5F00
}

// lssDevice starts a stack's unconfigured LSS slave (node-ID 255).
func lssDevice(t *testing.T, a, ifc string, serial uint32) *container {
	t.Helper()
	dev := startContainer(t, a, "device", "--interface", ifc, "--node-id", "255", "--eds", deviceEDS,
		"--serial", fmt.Sprintf("0x%08X", serial), "--timeout-ms", "900000")
	dev.await(t, `"type":"ready"`, 90*time.Second)
	return dev
}

// nodeIDEvent is the device's event when node-ID id became active.
func nodeIDEvent(id int) []string {
	return []string{`"type":"node-id"`, fmt.Sprintf(`"node":%d`, id)}
}

// serialOf reads 0x1018:04 of a node by SDO.
func serialOf(t *testing.T, ifc string, id int) (uint32, result) {
	t.Helper()
	r := ctl(t, "sdo", "read", fmt.Sprint(id), "0x1018", "4", "-t", "u32", "-i", ifc, "--format", "json")
	if r.code != 0 {
		return 0, r
	}
	v, _ := resultOf(t, r)["value"].(float64)
	return uint32(v), r
}

// TestLSSStackSlave: canopenctl's LSS master (lss) against each stack's LSS
// slave (device --node-id 255 --serial N): identify, select, inquire, bit
// timing, configure node-ID and store, Fastscan of unconfigured and
// configured devices, and assign-all.
func TestLSSStackSlave(t *testing.T) {
	ifc := needVCAN(t)
	names := []string{"lss-identify-non-configured", "lss-identify-remote", "lss-select", "lss-select-none", "lss-inquire",
		"lss-bit-timing", "lss-configure-node-id", "lss-activate-on-waiting", "lss-inquire-node-id", "lss-store",
		"lss-fastscan", "lss-fastscan-configured", "lss-assign-all"}
	for _, a := range adapters {
		t.Run(a, func(t *testing.T) {
			c := needImage(t, a)
			if !c.op("device") || !c.Features["lssSlave"] {
				for _, n := range names {
					scenario(t, a, n, func(t *testing.T) { capSkip(t, "%s's device has no LSS slave (lssSlave false)", a) })
				}
				return
			}
			lssCtl := func(t *testing.T, args ...string) result {
				return ctl(t, append(append([]string{"lss"}, args...), "-i", ifc, "--format", "json")...)
			}
			base := lssSerial(a, 0)
			addr := lssAddr(base)
			t.Run("unconfigured", func(t *testing.T) {
				dev := lssDevice(t, a, ifc, base)
				scenario(t, a, "lss-identify-non-configured", func(t *testing.T) {
					r := lssCtl(t, "identify")
					want(t, r, 0)
					if resultOf(t, r)["found"] != true {
						differ(t, a, "lss-identify-non-configured", "lss identify: no answer from the unconfigured slave (found false)")
					}
				})
				scenario(t, a, "lss-identify-remote", func(t *testing.T) {
					lo, hi := lssAddr(base-1), lssAddr(base+1)
					r := lssCtl(t, "identify", "--address", lo, "--to", hi)
					want(t, r, 0)
					if resultOf(t, r)["found"] != true {
						differ(t, a, "lss-identify-remote", "lss identify --address %s --to %s: no answer (found false)", lo, hi)
						return
					}
					r = lssCtl(t, "identify", "--address", lssAddr(base+0x1000), "--to", lssAddr(base+0x2000))
					want(t, r, 0)
					if resultOf(t, r)["found"] != false {
						t.Fatalf("a slave outside the range answered: %s", r.out)
					}
				})
				scenario(t, a, "lss-select", func(t *testing.T) {
					want(t, lssCtl(t, "select", addr), 0, `"state": "configuration"`)
					want(t, lssCtl(t, "switch", "waiting"), 0)
				})
				scenario(t, a, "lss-select-none", func(t *testing.T) {
					want(t, lssCtl(t, "select", lssAddr(0x7FFFFFFF)), 5, "no_response")
				})
				scenario(t, a, "lss-inquire", func(t *testing.T) {
					r := lssCtl(t, "inquire", "--address", addr)
					want(t, r, 0)
					res := resultOf(t, r)
					ad, _ := res["address"].(map[string]any)
					if ad["address"] != addr || res["nodeId"] != 255.0 || res["configured"] != false {
						t.Fatalf("inquire: %s; want %s and node-ID 255", r.out, addr)
					}
				})
				scenario(t, a, "lss-bit-timing", func(t *testing.T) {
					if !c.Features["lssSlaveBitTiming"] {
						capSkip(t, "%s's LSS slave has no bit timing (lssSlaveBitTiming false)", a)
					}
					want(t, lssCtl(t, "configure", "--address", addr, "--bit-rate", "250"), 0, `"bitRate"`)
					want(t, lssCtl(t, "switch", "configuration"), 0)
					want(t, lssCtl(t, "activate", "--delay", "50ms"), 0)
					time.Sleep(150 * time.Millisecond) // the switch delay, then silence for another
					want(t, lssCtl(t, "switch", "waiting"), 0)
					if !dev.seen(5*time.Second, `"type":"bit-rate"`, `"kbps":250`, `"delayMs":50`) {
						t.Fatalf("no bit-rate event (250 kbit/s after 50 ms): %s", dev.stdout())
					}
				})
				configured, byItself := false, false
				scenario(t, a, "lss-configure-node-id", func(t *testing.T) {
					want(t, lssCtl(t, "configure", "--address", addr, "--node-id", "40"), 0, `"nodeId": 40`)
					byItself = dev.seen(3*time.Second, nodeIDEvent(40)...)
					if !byItself {
						want(t, ctl(t, "nmt", "reset-comm", "all", "-i", ifc), 0)
						if !dev.seen(5*time.Second, nodeIDEvent(40)...) {
							t.Fatalf("node-ID 40 not active, also after NMT reset communication: %s", dev.stdout())
						}
					}
					var s uint32
					var r result
					for i := 0; i < 20; i++ { // the node boots after the node-ID became active
						if s, r = serialOf(t, ifc, 40); r.code == 0 {
							break
						}
						time.Sleep(100 * time.Millisecond)
					}
					if r.code != 0 || s != base {
						t.Fatalf("node 40: serial %#x, want %#x\n%s%s", s, base, r.out, r.err)
					}
					configured = true
				})
				scenario(t, a, "lss-activate-on-waiting", func(t *testing.T) {
					if !configured {
						t.Fatal("not observed: lss-configure-node-id failed")
					}
					if !byItself {
						differ(t, a, "lss-activate-on-waiting", "no node-id event within 3 s of the switch to waiting; node-ID 40 active after nmt reset-comm")
					}
				})
				scenario(t, a, "lss-inquire-node-id", func(t *testing.T) {
					r := lssCtl(t, "inquire", "--address", addr)
					want(t, r, 0)
					if res := resultOf(t, r); res["nodeId"] != 40.0 || res["configured"] != true {
						t.Fatalf("inquire: %s; want node-ID 40", r.out)
					}
				})
				scenario(t, a, "lss-store", func(t *testing.T) {
					if !c.Features["lssSlaveStore"] {
						capSkip(t, "%s's LSS slave has no store (lssSlaveStore false)", a)
					}
					want(t, lssCtl(t, "configure", "--address", addr, "--store"), 0, `"stored": true`)
					if res := dev.stop(t); res["nodeId"] != 40.0 {
						t.Fatalf("device result %v, want nodeId 40", res)
					}
				})
			})
			t.Run("fastscan", func(t *testing.T) {
				dev := lssDevice(t, a, ifc, base+1)
				scenario(t, a, "lss-fastscan", func(t *testing.T) {
					r := lssCtl(t, append([]string{"fastscan"}, lssKnown...)...)
					want(t, r, 0)
					list, _ := resultOf(t, r)["devices"].([]any)
					if len(list) != 1 || list[0].(map[string]any)["address"] != lssAddr(base+1) {
						t.Fatalf("fastscan: %s; want [%s]", r.out, lssAddr(base+1))
					}
					// The whole address scanned (no known parts).
					r = lssCtl(t, "fastscan", "--max", "1")
					want(t, r, 0, lssAddr(base+1))
					_ = dev
				})
			})
			t.Run("configured", func(t *testing.T) {
				startContainer(t, a, "device", "--interface", ifc, "--node-id", "5", "--eds", deviceEDS,
					"--serial", fmt.Sprintf("0x%08X", base+2), "--timeout-ms", "900000").await(t, `"type":"ready"`, 90*time.Second)
				scenario(t, a, "lss-fastscan-configured", func(t *testing.T) {
					r := lssCtl(t, append([]string{"fastscan"}, lssKnown...)...)
					want(t, r, 0)
					if list, _ := resultOf(t, r)["devices"].([]any); len(list) != 0 {
						differ(t, a, "lss-fastscan-configured", "the configured node 5 (serial %#x) takes part in Fastscan: %v", base+2, list)
						return
					}
					r = lssCtl(t, "identify")
					want(t, r, 0)
					if resultOf(t, r)["found"] != false {
						t.Fatalf("the configured node 5 answers identify non-configured: %s", r.out)
					}
				})
			})
			t.Run("assign-all", func(t *testing.T) {
				var devs []*container
				for i := uint32(3); i <= 4; i++ {
					devs = append(devs, lssDevice(t, a, ifc, base+i))
				}
				scenario(t, a, "lss-assign-all", func(t *testing.T) {
					args := append([]string{"assign-all", "--first", "126"}, lssKnown...)
					if c.Features["lssSlaveStore"] {
						args = append(args, "--store")
					}
					r := lssCtl(t, args...)
					want(t, r, 0)
					list, _ := resultOf(t, r)["assignments"].([]any)
					var got []string
					for _, x := range list {
						e := x.(map[string]any)
						got = append(got, fmt.Sprintf("%v=%v", e["nodeId"], e["address"]))
					}
					sort.Strings(got)
					exp := []string{"126=" + lssAddr(base+3), "127=" + lssAddr(base+4)}
					if fmt.Sprint(got) != fmt.Sprint(exp) {
						t.Fatalf("assigned %v, want %v\n%s", got, exp, r.out)
					}
					reset := false
					for i, dev := range devs {
						id := 126 + i
						if !dev.seen(3*time.Second, nodeIDEvent(id)...) && !reset {
							// Lely: pending until NMT reset communication (lss-activate-on-waiting).
							want(t, ctl(t, "nmt", "reset-comm", "all", "-i", ifc), 0)
							reset = true
						}
						if !dev.seen(5*time.Second, nodeIDEvent(id)...) {
							t.Fatalf("serial %#x: node-ID %d not active: %s", base+3+uint32(i), id, dev.stdout())
						}
					}
					for i := range devs {
						id := 126 + i
						if s, r := serialOf(t, ifc, id); r.code != 0 || s != base+3+uint32(i) {
							t.Fatalf("node %d: serial %#x\n%s%s", id, s, r.out, r.err)
						}
					}
				})
			})
		})
	}
}

// TestLSSStackMaster: each stack's LSS master (lss) against canopenctl
// simulate --lss (go-canopen's LSS slave with the shared EDS, unconfigured
// or configured).
func TestLSSStackMaster(t *testing.T) {
	ifc := needVCAN(t)
	names := []string{"stack-lss-inquire", "stack-lss-configure-node-id", "stack-lss-store", "stack-lss-fastscan",
		"stack-lss-fastscan-configured", "stack-lss-assign-all"}
	for _, a := range adapters {
		t.Run(a, func(t *testing.T) {
			c := needImage(t, a)
			if !c.op("lss") || !c.Features["lssMaster"] {
				for _, n := range names {
					scenario(t, a, n, func(t *testing.T) { capSkip(t, "%s has no LSS master (lss)", a) })
				}
				return
			}
			dir := t.TempDir()
			fx := fixture(t, a, "eds/interop-device.eds", dir)
			base := lssSerial(a, 1)
			sel := func(serial uint32) string {
				return fmt.Sprintf("switch-selective:0x%08X:0x%08X:0x%08X:0x%08X", lssVendor, lssProduct, lssRevision, serial)
			}
			lssRun := func(t *testing.T, steps ...string) (int, string, []map[string]any) {
				args := []string{"lss", "--interface", ifc, "--timeout-ms", "170000"}
				for _, s := range steps {
					args = append(args, "--step", s)
				}
				code, out := adapterRunIn(t, a, "", 3*time.Minute, args...)
				var ev []map[string]any
				for _, d := range docs(t, out) {
					if d["type"] == "lss" {
						ev = append(ev, d)
					}
				}
				return code, out, ev
			}
			sim := func(t *testing.T, serial uint32, extra ...string) *ctlProc {
				p := startCtl(t, append([]string{"simulate", "-i", ifc, "--eds", fx, "--lss", "--serial", fmt.Sprintf("0x%08X", serial),
					"--heartbeat", "0", "--format", "json"}, extra...)...)
				p.await(t, `"type":"ready"`, 10*time.Second)
				return p
			}
			// active checks that node id answers SDO with the serial number.
			active := func(t *testing.T, id int, serial uint32) {
				t.Helper()
				var s uint32
				var r result
				for i := 0; i < 30; i++ {
					if s, r = serialOf(t, ifc, id); r.code == 0 {
						break
					}
					time.Sleep(100 * time.Millisecond)
				}
				if r.code != 0 || s != serial {
					t.Fatalf("node %d: serial %#x, want %#x\n%s%s", id, s, serial, r.out, r.err)
				}
			}
			t.Run("unconfigured", func(t *testing.T) {
				store := filepath.Join(dir, "lss.store")
				p := sim(t, base, "--store", store)
				scenario(t, a, "stack-lss-inquire", func(t *testing.T) {
					code, out, ev := lssRun(t, "switch-global:configuration", "inquire:vendor-id", "inquire:product-code",
						"inquire:revision", "inquire:serial", "inquire:node-id", "switch-global:waiting")
					var vals []string
					var node any
					for _, e := range ev {
						if e["step"] == "inquire" {
							if v, ok := e["value"].(string); ok {
								vals = append(vals, v)
							} else {
								node = e["node"]
							}
						}
					}
					exp := []string{fmt.Sprintf("0x%08X", lssVendor), fmt.Sprintf("0x%08X", lssProduct), fmt.Sprintf("0x%08X", lssRevision), fmt.Sprintf("0x%08X", base)}
					if code != 0 || fmt.Sprint(vals) != fmt.Sprint(exp) || node != 255.0 {
						t.Fatalf("exit %d: inquired %v node-ID %v; want %v and 255\n%s", code, vals, node, exp, out)
					}
				})
				scenario(t, a, "stack-lss-configure-node-id", func(t *testing.T) {
					code, out, _ := lssRun(t, sel(base), "configure-node-id:40", "switch-global:waiting")
					if code != 0 {
						t.Fatalf("exit %d: %s", code, out)
					}
					active(t, 40, base)
				})
				scenario(t, a, "stack-lss-store", func(t *testing.T) {
					code, out, _ := lssRun(t, sel(base), "store", "switch-global:waiting")
					if code != 0 {
						t.Fatalf("exit %d: %s", code, out)
					}
					// The stored node-ID survives a restart of the simulated device.
					p.stop()
					q := startCtl(t, "simulate", "-i", ifc, "--eds", fx, "--lss", "--serial", fmt.Sprintf("0x%08X", base), "--store", store,
						"--heartbeat", "0", "--format", "json")
					q.await(t, `"type":"ready"`, 10*time.Second)
					if !q.seen(time.Second, `"type":"ready"`, `"node":40`) {
						t.Fatalf("restarted with the store: %s", q.stdout())
					}
					active(t, 40, base)
				})
			})
			t.Run("fastscan", func(t *testing.T) {
				sim(t, base+1)
				scenario(t, a, "stack-lss-fastscan", func(t *testing.T) {
					if !c.Features["lssFastscan"] {
						capSkip(t, "%s's LSS master has no Fastscan (lssFastscan false)", a)
					}
					code, out, ev := lssRun(t, "fastscan", "configure-node-id:41", "switch-global:waiting")
					if code != 0 || len(ev) == 0 {
						t.Fatalf("exit %d: %s", code, out)
					}
					ad, _ := ev[0]["address"].(map[string]any)
					if ad["serial"] != fmt.Sprintf("0x%08X", base+1) || ad["productCode"] != fmt.Sprintf("0x%08X", lssProduct) {
						t.Fatalf("found %v, want serial 0x%08X", ad, base+1)
					}
					active(t, 41, base+1)
				})
			})
			t.Run("configured", func(t *testing.T) {
				sim(t, base+2, "--node-id", "5")
				scenario(t, a, "stack-lss-fastscan-configured", func(t *testing.T) {
					if !c.Features["lssFastscan"] {
						capSkip(t, "%s's LSS master has no Fastscan (lssFastscan false)", a)
					}
					code, out, ev := lssRun(t, "fastscan")
					if code != 3 {
						t.Fatalf("fastscan with only the configured node 5 on the bus: exit %d %v, want exit 3\n%s", code, ev, out)
					}
				})
			})
			t.Run("assign-all", func(t *testing.T) {
				if c.Features["lssAssignAll"] {
					// With a store: assign-all:FIRST:store makes each slave store
					// (without one go-canopen's slave refuses with error 1).
					for i := uint32(3); i <= 4; i++ {
						sim(t, base+i, "--store", filepath.Join(dir, fmt.Sprintf("assign%d.store", i)))
					}
				}
				scenario(t, a, "stack-lss-assign-all", func(t *testing.T) {
					if !c.Features["lssAssignAll"] {
						capSkip(t, "%s's LSS master has no assign-all service (lssAssignAll false)", a)
					}
					code, out, ev := lssRun(t, "assign-all:126:store")
					if code != 0 || len(ev) == 0 {
						t.Fatalf("exit %d: %s", code, out)
					}
					list, _ := ev[0]["assigned"].([]any)
					var got []string
					for _, x := range list {
						e := x.(map[string]any)
						ad, _ := e["address"].(map[string]any)
						got = append(got, fmt.Sprintf("%v:%v", e["node"], ad["serial"]))
					}
					sort.Strings(got)
					exp := []string{fmt.Sprintf("126:0x%08X", base+3), fmt.Sprintf("127:0x%08X", base+4)}
					if fmt.Sprint(got) != fmt.Sprint(exp) {
						t.Fatalf("assigned %v, want %v\n%s", got, exp, out)
					}
					active(t, 126, base+3)
					active(t, 127, base+4)
				})
			})
		})
	}
}
