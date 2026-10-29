// SPDX-License-Identifier: MIT

//go:build interop

package interop

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const deviceEDS = "/fixtures/eds/interop-device.eds"

// longText is 64 characters: the size every stack gives 0x2004.
const longText = "canopenctl interop test string of exactly sixty-four characters!"

// TestStackDevice: canopenctl as the client, NMT master and consumer of
// each stack's device (node 5, the shared interop-device.eds).
func TestStackDevice(t *testing.T) {
	ifc := needVCAN(t)
	for _, a := range adapters {
		t.Run(a, func(t *testing.T) {
			c := needImage(t, a)
			dir := t.TempDir()
			fx := fixture(t, a, "eds/interop-device.eds", dir)
			dcf := fixture(t, a, "eds/interop-device-node5.dcf", dir)
			args := []string{"device", "--interface", ifc, "--node-id", "5", "--eds", deviceEDS, "--timeout-ms", "180000",
				"--dump", "0x2000:00", "--dump", "0x2001:00", "--dump", "0x2004:00", "--dump", "0x2005:00"}
			if c.Features["heartbeatProducer"] {
				args = append(args, "--heartbeat-ms", "100")
			}
			if c.Features["tick"] {
				args = append(args, "--tick-ms", "20")
			}
			if c.Features["emcy"] {
				args = append(args, "--emcy", "1000:01:2b01020304", "--emcy-after-ms", "4000")
			}
			dev := startContainer(t, a, args...)
			dev.await(t, `"type":"ready"`, 60*time.Second)
			var emcyW, hb *ctlProc
			if c.Features["emcy"] {
				emcyW = startCtl(t, "emcy", "watch", "-i", ifc, "--node", "5", "--count", "1", "--duration", "60s", "--format", "json")
			}
			if c.Features["heartbeatProducer"] {
				hb = startCtl(t, "heartbeat", "watch", "-i", ifc, "--node", "5", "--supervise", "1s", "--duration", "170s")
			}
			sdoRead := func(t *testing.T, args ...string) result {
				return ctl(t, append([]string{"sdo", "read", "5", "-i", ifc, "--eds", fx}, args...)...)
			}
			scenario(t, a, "sdo-read-expedited", func(t *testing.T) {
				want(t, sdoRead(t, "0x2001", "0"), 0, "8755 (0x2233)")
			})
			scenario(t, a, "sdo-read-segmented", func(t *testing.T) {
				want(t, sdoRead(t, "0x1008", "0"), 0, `"otfabric canopen-interop device"`)
			})
			scenario(t, a, "sdo-read-block", func(t *testing.T) {
				if !c.Features["deviceBlock"] {
					capSkip(t, "%s's device has no SDO block transfer", a)
				}
				want(t, sdoRead(t, "0x1008", "0", "--block"), 0, `"otfabric canopen-interop device"`, "")
			})
			scenario(t, a, "sdo-write-expedited", func(t *testing.T) {
				want(t, ctl(t, "sdo", "write", "5", "0x2000", "0", "0x5A", "--eds", fx, "-i", ifc, "--verify"), 0, "verified")
			})
			scenario(t, a, "sdo-write-segmented", func(t *testing.T) {
				want(t, ctl(t, "sdo", "write", "5", "0x2004", "0", longText, "--eds", fx, "-i", ifc, "--verify"), 0, "verified")
			})
			scenario(t, a, "sdo-domain-segmented-and-block", func(t *testing.T) {
				data := bytes.Repeat([]byte("0123456789"), 100)
				in := filepath.Join(dir, "domain.bin")
				_ = os.WriteFile(in, data, 0o644)
				blk := []string{}
				if c.Features["deviceBlock"] {
					blk = []string{"--block"}
				}
				want(t, ctl(t, append([]string{"sdo", "write", "5", "0x2005", "0", "--file", in, "-i", ifc}, blk...)...), 0, "written")
				out := filepath.Join(dir, "back.bin")
				want(t, ctl(t, append([]string{"sdo", "read", "5", "0x2005", "0", "-o", out, "-i", ifc}, blk...)...), 0)
				if b, _ := os.ReadFile(out); !bytes.Equal(b, data) {
					t.Fatalf("read back %d bytes", len(b))
				}
			})
			scenario(t, a, "sdo-abort-write-only", func(t *testing.T) {
				want(t, sdoRead(t, "0x2009", "0"), 11, "0x06010001")
			})
			scenario(t, a, "sdo-abort-read-only", func(t *testing.T) {
				want(t, ctl(t, "sdo", "write", "5", "0x2008", "0", "1", "--eds", fx, "-i", ifc), 11, "0x06010002")
			})
			scenario(t, a, "sdo-abort-no-object", func(t *testing.T) {
				want(t, ctl(t, "sdo", "read", "5", "0x5FFF", "0", "-i", ifc), 11, "0x06020000")
			})
			scenario(t, a, "sdo-abort-value-range", func(t *testing.T) {
				if !c.Features["deviceLimits"] {
					capSkip(t, "%s's device does not check limits", a)
				}
				want(t, ctl(t, "sdo", "write", "5", "0x200A", "0", "5", "--eds", fx, "-i", ifc), 11, "0x06090032")
			})
			scenario(t, a, "sdo-dump", func(t *testing.T) {
				r := ctl(t, "sdo", "dump", "5", "--eds", fx, "-i", ifc, "--range", "2000-200A")
				want(t, r, 0, "0x2001:00", "0x2008:00")
			})
			scenario(t, a, "scan", func(t *testing.T) {
				want(t, ctl(t, "scan", "-i", ifc, "--nodes", "1-8"), 0, "node   5", "product 0x434F4931")
			})
			scenario(t, a, "pdo-show", func(t *testing.T) {
				want(t, ctl(t, "pdo", "show", "5", "--tpdo", "1", "--eds", fx, "-i", ifc), 0, "TPDO1  COB-ID 0x185 (valid)  active", "0x2002:00/32")
			})
			scenario(t, a, "nmt-start", func(t *testing.T) {
				if c.Features["heartbeatProducer"] {
					want(t, ctl(t, "nmt", "start", "5", "-i", ifc, "--wait"), 0, "reports operational")
				} else {
					want(t, ctl(t, "nmt", "start", "5", "-i", ifc), 0)
				}
			})
			scenario(t, a, "pdo-watch-tpdo1", func(t *testing.T) {
				if !c.Features["tick"] || !c.Features["deviceTpdo"] {
					capSkip(t, "%s's device produces no TPDO", a)
				}
				want(t, ctl(t, "pdo", "watch", "5", "--tpdo", "1", "--eds", fx, "-i", ifc, "--count", "2", "--duration", "10s"), 0, "TPDO1 185#", "Test counter")
			})
			scenario(t, a, "pdo-map-tpdo2", func(t *testing.T) {
				if !c.Features["deviceTpdo"] {
					capSkip(t, "%s's device produces no TPDO", a)
				}
				want(t, ctl(t, "nmt", "preop", "5", "-i", ifc), 0)
				want(t, ctl(t, "pdo", "map", "5", "--tpdo", "2", "--map", "0x2000:00:8,0x2001:00:16", "--type", "255", "--event-timer", "50", "--eds", fx, "-i", ifc),
					0, "TPDO2  COB-ID 0x285 (valid)  active")
				want(t, ctl(t, "nmt", "start", "5", "-i", ifc), 0)
				want(t, ctl(t, "pdo", "watch", "5", "--tpdo", "2", "--eds", fx, "-i", ifc, "--count", "2", "--duration", "10s", "--format", "json"),
					0, `"key":"0x2000:00"`, `"value":90`, `"value":8755`)
			})
			scenario(t, a, "emcy-watch", func(t *testing.T) {
				if emcyW == nil {
					capSkip(t, "%s's device sends no EMCY on request", a)
				}
				want(t, emcyW.wait(t, 60*time.Second), 0, `"code":"0x1000"`, `"node":5`, `"msef":"2b01020304"`)
			})
			scenario(t, a, "heartbeat-watch", func(t *testing.T) {
				if hb == nil {
					capSkip(t, "%s's device has no heartbeat producer", a)
				}
				time.Sleep(300 * time.Millisecond)
				hb.stop()
				if !strings.Contains(hb.stdout(), "node 5 pre-operational -> operational") {
					t.Fatalf("heartbeat watch:\n%s\n%s", hb.stdout(), hb.errb.String())
				}
			})
			scenario(t, a, "dcf-apply", func(t *testing.T) {
				want(t, ctl(t, "nmt", "preop", "5", "-i", ifc), 0)
				want(t, ctl(t, "dcf", "apply", "5", dcf, "-i", ifc), 0, "written to node 5")
				want(t, sdoRead(t, "0x2001", "0"), 0, "(0x4455)")
			})
			scenario(t, a, "device-dump", func(t *testing.T) {
				res := dev.stop(t)
				vals, _ := res["values"].(map[string]any)
				if vals["0x2000:00"] != "5a" {
					t.Fatalf("0x2000:00 = %v (want 5a)\n%v", vals["0x2000:00"], res)
				}
				if vals["0x2001:00"] != "5544" {
					t.Fatalf("0x2001:00 = %v (want 5544 after dcf apply)", vals["0x2001:00"])
				}
			})
		})
	}
}

// TestStackClient: each stack's client against canopenctl simulate with
// the shared interop-device.eds.
func TestStackClient(t *testing.T) {
	ifc := needVCAN(t)
	for _, a := range adapters {
		t.Run(a, func(t *testing.T) {
			c := needImage(t, a)
			dir := t.TempDir()
			fx := fixture(t, a, "eds/interop-device.eds", dir)
			sim := startCtl(t, "simulate", "-i", ifc, "--eds", fx, "--node-id", "6", "--heartbeat", "100", "--tick", "2002:00", "--tick-period", "20ms", "--format", "json")
			sim.await(t, `"type":"ready"`, 10*time.Second)
			up := func(t *testing.T, args ...string) (int, map[string]any) {
				code, out := adapterRun(t, a, append([]string{"sdo-upload", "--interface", ifc, "--node-id", "6"}, args...)...)
				return code, lastDoc(t, out)
			}
			scenario(t, a, "stack-sdo-upload-expedited", func(t *testing.T) {
				code, d := up(t, "--index", "0x2001")
				if code != 0 || d["data"] != "3322" {
					t.Fatalf("exit %d: %v", code, d)
				}
			})
			scenario(t, a, "stack-sdo-upload-segmented", func(t *testing.T) {
				code, d := up(t, "--index", "0x1008")
				if code != 0 || d["data"] != hex.EncodeToString([]byte("otfabric canopen-interop device")) {
					t.Fatalf("exit %d: %v", code, d)
				}
			})
			scenario(t, a, "stack-sdo-download", func(t *testing.T) {
				code, out := adapterRun(t, a, "sdo-download", "--interface", ifc, "--node-id", "6", "--index", "0x2000", "--data", "7b")
				if code != 0 {
					t.Fatalf("exit %d: %s", code, out)
				}
				want(t, ctl(t, "sdo", "read", "6", "0x2000", "0", "-i", ifc, "-t", "u8"), 0, "123 (0x7B)")
			})
			scenario(t, a, "stack-sdo-block", func(t *testing.T) {
				if !c.Features["sdoBlock"] {
					capSkip(t, "%s's client has no block transfer", a)
				}
				data := hex.EncodeToString(bytes.Repeat([]byte{0xA5, 0x5A, 0x01}, 100))
				code, out := adapterRun(t, a, "sdo-download", "--interface", ifc, "--node-id", "6", "--index", "0x2005", "--data", data, "--block")
				if code != 0 {
					t.Fatalf("download exit %d: %s", code, out)
				}
				code, d := up(t, "--index", "0x2005", "--block")
				if code != 0 || d["data"] != data {
					t.Fatalf("upload exit %d: %v", code, d)
				}
			})
			scenario(t, a, "stack-sdo-abort", func(t *testing.T) {
				code, d := up(t, "--index", "0x2009")
				if code != 5 || d["abortCode"] != "0x06010001" {
					t.Fatalf("exit %d: %v", code, d)
				}
			})
			scenario(t, a, "stack-nmt-start", func(t *testing.T) {
				code, out := adapterRun(t, a, "nmt", "--interface", ifc, "--node-id", "6", "--command", "start")
				if code != 0 {
					t.Fatalf("exit %d: %s", code, out)
				}
				sim.await(t, `"to":"operational"`, 10*time.Second)
			})
			scenario(t, a, "stack-pdo-receive", func(t *testing.T) {
				if !c.op("pdo-receive") {
					capSkip(t, "%s has no pdo-receive", a)
				}
				want(t, ctl(t, "nmt", "start", "6", "-i", ifc), 0)
				code, out := adapterRun(t, a, "pdo-receive", "--interface", ifc, "--node-id", "6", "--tpdo", "1", "--count", "2", "--timeout-ms", "10000")
				if code != 0 || !strings.Contains(out, `"0x2002:00"`) {
					t.Fatalf("exit %d: %s", code, out)
				}
			})
			scenario(t, a, "stack-pdo-configure", func(t *testing.T) {
				if !c.op("pdo-configure") {
					capSkip(t, "%s has no pdo-configure", a)
				}
				want(t, ctl(t, "nmt", "preop", "6", "-i", ifc), 0)
				code, out := adapterRun(t, a, "pdo-configure", "--interface", ifc, "--node-id", "6", "--tpdo", "2", "--map", "0x2002:00:32",
					"--transmission-type", "255", "--event-timer-ms", "50")
				if code != 0 {
					t.Fatalf("exit %d: %s", code, out)
				}
				want(t, ctl(t, "pdo", "show", "6", "--tpdo", "2", "-i", ifc), 0, "TPDO2  COB-ID 0x286 (valid)  active  type 255", "event timer 50 ms", "0x2002:00/32")
			})
			scenario(t, a, "stack-sync", func(t *testing.T) {
				if !c.op("sync") {
					capSkip(t, "%s has no sync", a)
				}
				mon := startCtl(t, "monitor", "-i", ifc, "--service", "sync", "--count", "3", "--duration", "30s")
				time.Sleep(300 * time.Millisecond)
				code, out := adapterRun(t, a, "sync", "--interface", ifc, "--count", "3", "--period-ms", "10")
				if code != 0 {
					t.Fatalf("exit %d: %s", code, out)
				}
				r := mon.wait(t, 30*time.Second)
				if r.code != 0 || strings.Count(r.out, "SYNC") != 3 {
					t.Fatalf("monitor: %+v", r)
				}
			})
			scenario(t, a, "stack-listen-bootup-heartbeat", func(t *testing.T) {
				if !c.op("listen") || !c.Features["listenBootup"] {
					capSkip(t, "%s cannot listen for boot-ups", a)
				}
				l := startContainer(t, a, "listen", "--interface", ifc, "--node-id", "6", "--count", "2", "--timeout-ms", "20000")
				l.await(t, `"type":"ready"`, 60*time.Second)
				want(t, ctl(t, "nmt", "reset-node", "6", "-i", ifc, "--wait"), 0)
				// The reset restored 0x1017 (0 in the EDS): heartbeats again.
				want(t, ctl(t, "sdo", "write", "6", "0x1017", "0", "100", "-t", "u16", "-i", ifc), 0)
				code, _ := l.wait(t, 30*time.Second)
				if code != 0 || !strings.Contains(l.stdout(), `"type":"bootup"`) {
					t.Fatalf("exit %d: %s", code, l.stdout())
				}
			})
			sim.stop()
			scenario(t, a, "stack-listen-emcy", func(t *testing.T) {
				if !c.op("listen") || !c.Features["listenEmcy"] {
					capSkip(t, "%s cannot listen for EMCY", a)
				}
				l := startContainer(t, a, "listen", "--interface", ifc, "--node-id", "8", "--count", "8", "--timeout-ms", "8000")
				l.await(t, `"type":"ready"`, 60*time.Second)
				s := startCtl(t, "simulate", "-i", ifc, "--node-id", "8", "--emcy", "2310:03:0102", "--emcy-after", "500ms", "--duration", "5s")
				_ = s
				_, _ = l.wait(t, 30*time.Second)
				out := l.stdout()
				if !strings.Contains(out, `"type":"emcy"`) || !strings.Contains(out, `"code":"0x2310"`) {
					t.Fatalf("listen: %s", out)
				}
			})
			scenario(t, a, "stack-cia402-enable", func(t *testing.T) {
				if !c.op("cia402") {
					capSkip(t, "%s has no cia402 client", a)
				}
				s := startCtl(t, "simulate", "-i", ifc, "--node-id", "7", "--drive")
				s.await(t, "node 7 running", 10*time.Second)
				code, out := adapterRun(t, a, "cia402", "--interface", ifc, "--node-id", "7", "--op", "enable", "--timeout-ms", "20000")
				if code != 0 || !strings.Contains(strings.ToUpper(out), "OPERATION ENABLED") {
					t.Fatalf("exit %d: %s", code, out)
				}
				want(t, ctl(t, "cia402", "state", "7", "-i", ifc), 0, "Operation enabled")
				s.stop()
			})
		})
	}
}

// edsDifferences are the fixtures canopenctl (go-canopen's eds.Validate)
// rejects and a stack's checker accepts, with the reason.
var edsDifferences = map[string]string{
	// None since go-canopen v0.1.0: a PDO count that does not match the
	// PDO objects is a warning there, as in lely and python canopen.
}

// TestEDSCheck: each stack's EDS checker and eds validate agree on the
// fixtures (valid or not).
func TestEDSCheck(t *testing.T) {
	files := []string{"interop-device.eds", "interop-device-node5.dcf", "compact-subobj.eds", "object-links.eds", "invalid-missing-datatype.eds"}
	for _, a := range adapters {
		t.Run(a, func(t *testing.T) {
			c := needImage(t, a)
			dir := t.TempDir()
			for _, f := range files {
				scenario(t, a, "eds-"+strings.TrimSuffix(f, filepath.Ext(f)), func(t *testing.T) {
					if !c.op("eds-check") {
						capSkip(t, "%s has no eds-check", a)
					}
					local := fixture(t, a, "eds/"+f, dir)
					_, out := adapterRun(t, a, "eds-check", "--eds", "/fixtures/eds/"+f)
					d := lastDoc(t, out)
					theirs, _ := d["valid"].(bool)
					r := ctl(t, "eds", "validate", local)
					ours := r.code == 0
					if r.code != 0 && r.code != 1 {
						t.Fatalf("eds validate exit %d: %s", r.code, r.err)
					}
					if ours != theirs {
						if why, ok := edsDifferences[f]; ok && !ours && theirs {
							knownDifference(t, why)
							return
						}
						t.Fatalf("%s says valid=%v, canopenctl valid=%v\n%s\n%s", a, theirs, ours, out, r.out)
					}
				})
			}
		})
	}
}
