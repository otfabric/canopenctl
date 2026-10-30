// SPDX-License-Identifier: MIT

//go:build interop

package interop

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// mpdoEDS is canopen-interop's MPDO device: the shared test device with an
// object scanner list 0x1FA0 and an object dispatching list 0x1FD0 (4
// entries each).
const mpdoEDS = "/fixtures/eds/mpdo-device.eds"

// TestMPDO: canopenctl's mpdo commands with each stack's MPDO devices:
// mpdo configure (RPDO 1 as DAM or SAM consumer, TPDO 1 as SAM producer),
// mpdo dispatcher and scanner on a stack device, checked by the MPDOs of
// the stack's producer (mpdo-send) and by SDO; mpdo watch decoding the
// stack's DAM- and SAM-MPDOs.
func TestMPDO(t *testing.T) {
	ifc := needVCAN(t)
	for _, a := range adapters {
		t.Run(a, func(t *testing.T) {
			c := needImage(t, a)
			need := func(t *testing.T, op, mode string) {
				t.Helper()
				feature := map[string]string{"dam": "mpdoDam", "sam": "mpdoSam"}[mode]
				if !c.op(op) || !c.Features[feature] {
					capSkip(t, "%s has no %s in %s mode (%s false)", a, op, mode, feature)
				}
			}
			send := func(t *testing.T, producer int, mode, cob, index, data string, target int) {
				t.Helper()
				args := []string{"mpdo-send", "--interface", ifc, "--node-id", fmt.Sprint(producer), "--mode", mode, "--cob-id", cob,
					"--index", index, "--data", data}
				if target >= 0 {
					args = append(args, "--target", fmt.Sprint(target))
				}
				if code, out := adapterRun(t, a, args...); code != 0 {
					t.Fatalf("mpdo-send %v: exit %d: %s", args[3:], code, out)
				}
			}
			// A stack device of the MPDO EDS, node id, pre-operational.
			device := func(t *testing.T, id int) *container {
				t.Helper()
				dev := startContainer(t, a, "device", "--interface", ifc, "--node-id", fmt.Sprint(id), "--eds", mpdoEDS,
					"--timeout-ms", "900000", "--dump", "0x2000:00", "--dump", "0x2001:00")
				dev.await(t, `"type":"ready"`, 90*time.Second)
				return dev
			}
			read := func(t *testing.T, id int, index string, typ string) string {
				t.Helper()
				r := ctl(t, "sdo", "read", fmt.Sprint(id), index, "0", "-t", typ, "-i", ifc, "--format", "json")
				want(t, r, 0)
				d, _ := resultOf(t, r)["data"].(string)
				return d
			}
			scenario(t, a, "mpdo-configure-dam", func(t *testing.T) {
				need(t, "mpdo-send", "dam")
				device(t, 10)
				want(t, ctl(t, "mpdo", "configure", "10", "--rpdo", "1", "--mode", "dam", "--cob-id", "0x300", "-i", ifc),
					0, "node 10 RPDO1 is an MPDO, DAM, COB-ID 0x300")
				want(t, ctl(t, "pdo", "show", "10", "--rpdo", "1", "-i", ifc), 0, "COB-ID 0x300 (valid)")
				want(t, ctl(t, "nmt", "start", "10", "-i", ifc), 0)
				time.Sleep(200 * time.Millisecond)
				send(t, 11, "dam", "0x300", "0x2001", "efbe", 10)
				send(t, 11, "dam", "0x300", "0x2000", "99", 12) // another node: ignored
				if got := read(t, 10, "0x2001", "u16"); got != "efbe" {
					t.Fatalf("0x2001 = %s after a DAM-MPDO to node 10, want efbe", got)
				}
				if got := read(t, 10, "0x2000", "u8"); got != "11" {
					t.Fatalf("0x2000 = %s after a DAM-MPDO to node 12, want the default 11", got)
				}
				send(t, 11, "dam", "0x300", "0x2000", "42", 0) // all nodes
				if got := read(t, 10, "0x2000", "u8"); got != "42" {
					t.Fatalf("0x2000 = %s after a DAM-MPDO to all nodes, want 42", got)
				}
			})
			scenario(t, a, "mpdo-dispatcher-sam", func(t *testing.T) {
				need(t, "mpdo-send", "sam")
				device(t, 10)
				want(t, ctl(t, "mpdo", "configure", "10", "--rpdo", "1", "--mode", "sam", "--cob-id", "0x301", "-i", ifc),
					0, "node 10 RPDO1 is an MPDO, SAM, COB-ID 0x301")
				want(t, ctl(t, "mpdo", "dispatcher", "10", "--list", "1", "--entry", "11:2001:00=2001:00", "--entry", "12:2000:00=2000:00", "-i", ifc),
					0, "2 entries written")
				r := ctl(t, "sdo", "read", "10", "0x1FD0", "0", "-t", "u8", "-i", ifc)
				want(t, r, 0, "2 (0x02)")
				want(t, ctl(t, "nmt", "start", "10", "-i", ifc), 0)
				time.Sleep(200 * time.Millisecond)
				send(t, 11, "sam", "0x301", "0x2001", "3412", -1) // dispatched
				send(t, 13, "sam", "0x301", "0x2001", "5555", -1) // producer not listed
				send(t, 11, "sam", "0x301", "0x2000", "66", -1)   // object not listed for node 11
				send(t, 12, "sam", "0x301", "0x2000", "77", -1)   // dispatched
				if got := read(t, 10, "0x2001", "u16"); got != "3412" {
					t.Fatalf("0x2001 = %s, want 3412 (SAM-MPDO of node 11, dispatched)", got)
				}
				if got := read(t, 10, "0x2000", "u8"); got != "77" {
					t.Fatalf("0x2000 = %s, want 77 (SAM-MPDO of node 12, dispatched)", got)
				}
			})
			scenario(t, a, "mpdo-scanner", func(t *testing.T) {
				need(t, "mpdo-send", "sam")
				device(t, 11)
				want(t, ctl(t, "mpdo", "configure", "11", "--tpdo", "1", "--mode", "sam", "--cob-id", "0x301", "-i", ifc),
					0, "node 11 TPDO1 is an MPDO, SAM, COB-ID 0x301")
				want(t, ctl(t, "mpdo", "scanner", "11", "--list", "1", "--entry", "2001:00", "--entry", "2002:00+1", "-i", ifc),
					0, "2 entries written")
				// Entries IIII:SS with the block size in bits 24..31 (CiA 301), little-endian.
				for _, x := range [][2]string{{"0", "02"}, {"1", "00012000"}, {"2", "00022001"}} {
					r := ctl(t, "sdo", "read", "11", "0x1FA0", x[0], "-i", ifc, "--format", "json")
					want(t, r, 0)
					if got, _ := resultOf(t, r)["data"].(string); got != x[1] {
						t.Fatalf("0x1FA0:%s = %s, want %s", x[0], got, x[1])
					}
				}
				want(t, ctl(t, "pdo", "show", "11", "--tpdo", "1", "-i", ifc), 0, "COB-ID 0x301 (valid)")
			})
			watch := func(t *testing.T, mode, cob string, run func(t *testing.T)) string {
				t.Helper()
				w := startCtl(t, "mpdo", "watch", "--cob-id", cob, "-i", ifc, "--count", "1", "--duration", "60s", "--format", "json")
				time.Sleep(300 * time.Millisecond)
				run(t)
				r := w.wait(t, 60*time.Second)
				if r.code != 0 {
					t.Fatalf("mpdo watch: exit %d\n%s%s", r.code, r.out, r.err)
				}
				for _, d := range docs(t, r.out) {
					if d["type"] == "mpdo" {
						return fmt.Sprintf("%v %v 0x%04X:%02X %v", d["mode"], d["node"], int(d["index"].(float64)), int(d["subIndex"].(float64)), d["data"])
					}
				}
				t.Fatalf("no MPDO: %s", r.out)
				return ""
			}
			scenario(t, a, "mpdo-watch-dam", func(t *testing.T) {
				need(t, "mpdo-send", "dam")
				got := watch(t, "dam", "0x300", func(t *testing.T) { send(t, 11, "dam", "0x300", "0x2001", "efbe", 10) })
				if !strings.HasPrefix(got, "dam 10 0x2001:00 efbe") {
					t.Fatalf("mpdo watch: %s, want dam 10 0x2001:00 efbe..", got)
				}
				got = watch(t, "dam", "0x300", func(t *testing.T) { send(t, 11, "dam", "0x300", "0x2002", "78563412", 0) })
				if got != "dam 0 0x2002:00 78563412" {
					t.Fatalf("mpdo watch: %s, want dam 0 0x2002:00 78563412", got)
				}
			})
			scenario(t, a, "mpdo-watch-sam", func(t *testing.T) {
				need(t, "mpdo-send", "sam")
				got := watch(t, "sam", "0x301", func(t *testing.T) { send(t, 11, "sam", "0x301", "0x2001", "3412", -1) })
				if !strings.HasPrefix(got, "sam 11 0x2001:00 3412") {
					t.Fatalf("mpdo watch: %s, want sam 11 0x2001:00 3412..", got)
				}
			})
		})
	}
}
