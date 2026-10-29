// SPDX-License-Identifier: MIT

package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/otfabric/go-canopen"
	"github.com/otfabric/go-canopen/node"
	"github.com/otfabric/go-canopen/pdo"
)

func TestPDOShow(t *testing.T) {
	fresh(t)
	ioNode(t, 5)
	r := run(t, "", "pdo", "show", "5", "-i", "virtual", "--eds", "testdata/io401.eds")
	want(t, r, 0, "TPDO1  COB-ID 0x185 (valid)", "0x6000:01/8  Read input 8-bit / Read input 1h to 1h+7", "RPDO2")
	if strings.Contains(r.out, "TPDO3") {
		t.Fatal("io401 has 2 TPDOs")
	}
	r = run(t, "", "pdo", "show", "5", "-i", "virtual", "--tpdo", "2", "--format", "json")
	want(t, r, 0, `"kind": "tpdo"`, `"pdo": 2`, `"cobId": "0x285"`, `"transmissionType": 254`)
	golden(t, "pdo-show-tpdo2.json", r.out)
	want(t, run(t, "", "pdo", "show", "5", "-i", "virtual", "--rpdo", "1", "--format", "table"), 0, "RPDO1", "MAPPING")
	want(t, run(t, "", "pdo", "show", "5", "-i", "virtual", "--rpdo", "3"), 11, "0x06020000")
	want(t, run(t, "", "pdo", "show", "5", "-i", "virtual", "--rpdo", "1", "--tpdo", "1"), 3, "not both")
	want(t, run(t, "", "pdo", "show", "5", "-i", "virtual", "--rpdo", "600"), 3, "1..512")
	want(t, run(t, "", "pdo", "show", "x", "-i", "virtual"), 3, "invalid node-ID")
}

func TestPDOMapAndWatch(t *testing.T) {
	fresh(t)
	n := builtinNode(t, 7, false)
	// Map TPDO2 to the counter and the output, event timer 5 ms.
	r := run(t, "", "pdo", "map", "7", "--tpdo", "2", "--map", "0x2000:00:32,2001:00:8", "--type", "255", "--event-timer", "5", "-i", "virtual")
	want(t, r, 0, "node 7 configured:", "TPDO2  COB-ID 0x287 (valid)  active  type 255", "0x2000:00/32")
	cfg := n.TPDO(2).Config()
	if !cfg.Active() || len(cfg.Mapping) != 2 || cfg.EventTimer != 5 {
		t.Fatalf("%+v", cfg)
	}
	_ = n.SetState(canopen.StateOperational)
	_ = n.Dictionary().SetUint(0x2000, 0, 0x01020304)
	r = run(t, "", "pdo", "watch", "7", "--tpdo", "2", "-i", "virtual", "--count", "2", "--format", "json")
	want(t, r, 0, `"type":"pdo"`, `"cobId":"0x287"`, `"key":"0x2000:00"`, `"value":16909060`, `"type":"summary"`)
	r = run(t, "", "pdo", "watch", "7", "--tpdo", "2", "-i", "virtual", "--count", "1", "--cob-id", "287", "--map", "2000:00:16,2000:00:16,2001:00:8")
	want(t, r, 0, "TPDO2 287#", "0x2000:00=")
	// With the EDS of a device: names.
	want(t, run(t, "", "pdo", "map", "7", "--tpdo", "2", "--disable", "-i", "virtual", "--format", "json"), 0, `"valid": false`)
	want(t, run(t, "", "pdo", "watch", "7", "--tpdo", "2", "-i", "virtual"), 12, "not active")
	// RPDO with a dummy entry and a COB-ID.
	r = run(t, "", "pdo", "map", "7", "--rpdo", "2", "--map", "0005:00:8,2001:00:8", "--type", "1", "--cob-id", "301", "--no-rtr", "-i", "virtual")
	want(t, r, 0, "RPDO2  COB-ID 0x301 (valid)", "dummy UNSIGNED8")
	// Refusals and invalid input.
	want(t, run(t, "", "pdo", "map", "7", "--tpdo", "1", "--map", "0x2002:00:8", "-i", "virtual"), 11, "0x06040041")
	want(t, run(t, "", "pdo", "map", "7", "--tpdo", "1", "--map", "0x6000:01:8", "--eds", "testdata/io401.eds", "-i", "virtual"), 11, "0x06020000")
	want(t, run(t, "", "pdo", "map", "7", "--tpdo", "1", "--map", "0x2002:00:8", "--eds", "testdata/io401.eds", "-i", "virtual"), 3, "does not fit")
	want(t, run(t, "", "pdo", "map", "7", "--map", "2000:00:32", "-i", "virtual"), 3, "--tpdo or --rpdo")
	want(t, run(t, "", "pdo", "map", "7", "--tpdo", "1", "-i", "virtual"), 3, "--map")
	want(t, run(t, "", "pdo", "map", "7", "--tpdo", "1", "--map", "2000:00:32", "--type", "245", "-i", "virtual"), 3, "--type")
	want(t, run(t, "", "pdo", "map", "7", "--rpdo", "1", "--map", "2001:00:8", "--type", "253", "-i", "virtual"), 3, "--type")
	want(t, run(t, "", "pdo", "map", "7", "--rpdo", "1", "--map", "2001:00:8", "--inhibit", "3", "-i", "virtual"), 3, "TPDOs only")
	want(t, run(t, "", "pdo", "map", "7", "--tpdo", "1", "--map", "2000:00:32", "--inhibit", "70000", "-i", "virtual"), 3, "0..65535")
	want(t, run(t, "", "pdo", "map", "7", "--tpdo", "1", "--map", "2000:00:32", "--sync-start", "241", "-i", "virtual"), 3, "--sync-start")
	want(t, run(t, "", "pdo", "map", "7", "--tpdo", "1", "--map", "2000:00:32", "--cob-id", "605", "-i", "virtual"), 3, "reserved")
	want(t, run(t, "", "pdo", "map", "7", "--tpdo", "1", "--map", "2000:00:32", "--cob-id", "q", "-i", "virtual"), 3, "COB-ID")
	want(t, run(t, "", "pdo", "map", "7", "--tpdo", "900", "--map", "2000:00:32", "-i", "virtual"), 3, "1..512")
	want(t, run(t, "", "pdo", "map", "7", "--tpdo", "1", "--map", "2000:00:32", "-i", "slcan:/dev/null"), 9, "--yes")
	want(t, run(t, "", "pdo", "map", "7", "--tpdo", "1", "--map", "2000:00:99", "-i", "virtual"), 3, "1..64 bits")
	want(t, run(t, "", "pdo", "map", "x", "--tpdo", "1", "--map", "2000:00:32", "-i", "virtual"), 3, "node-ID")
}

func TestPDOWatchEDS(t *testing.T) {
	fresh(t)
	n := ioNode(t, 5, node.WithAutoStart())
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for i := uint64(0); ; i++ {
			select {
			case <-stop:
				return
			case <-time.After(5 * time.Millisecond):
			}
			_ = n.Dictionary().SetUint(0x6000, 1, i&0xFF)
		}
	}()
	r := run(t, "", "pdo", "watch", "5", "--tpdo", "1", "--eds", "testdata/io401.eds", "-i", "virtual", "--count", "2")
	want(t, r, 0, "node 5 TPDO1 185#", "Read input 8-bit / Read input 1h to 1h+7=", "2 PDOs received")
	want(t, run(t, "", "pdo", "watch", "5", "--tpdo", "1", "-i", "virtual", "--duration", "20ms", "--cob-id", "185"), 3, "go together")
	want(t, run(t, "", "pdo", "watch", "5", "--tpdo", "0", "-i", "virtual"), 3, "--tpdo")
	want(t, run(t, "", "pdo", "watch", "5", "--tpdo", "1", "-i", "virtual", "--duration", "20ms", "--cob-id", "185", "--map", "6000:01:12"), 3, "fit no data type")
	want(t, run(t, "", "pdo", "watch", "5", "--tpdo", "1", "-i", "virtual", "--duration", "20ms", "--cob-id", "185", "--map", "7000:01:8", "--eds", "testdata/io401.eds"), 3, "mapping")
	want(t, run(t, "", "pdo", "watch", "5", "--tpdo", "9", "-i", "virtual"), 11, "0x06020000")
}

func TestGuessDictionary(t *testing.T) {
	m := pdo.Mapping{{Index: 0x2000, SubIndex: 1, Bits: 1}, {Index: 0x2000, SubIndex: 2, Bits: 16}, {Index: 0x2000, SubIndex: 0, Bits: 8},
		{Index: 0x0002, Bits: 8}, {Index: 0x2001, Bits: 24}}
	d, err := guessDictionary(m)
	if err != nil {
		t.Fatal(err)
	}
	if v, err := d.Variable(0x2000, 2); err != nil || v.DataType != canopen.Unsigned16 {
		t.Fatal(v, err)
	}
	if _, err := pdo.NewDecoder(m, d); err != nil {
		t.Fatal(err)
	}
}
