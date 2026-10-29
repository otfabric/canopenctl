// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/otfabric/go-canopen"
	"github.com/otfabric/go-canopen/cdcf"
	"github.com/otfabric/go-canopen/emcy"
	"github.com/otfabric/go-canopen/node"
	"github.com/otfabric/go-canopen/od"
	"github.com/otfabric/go-canopen/program"
	"github.com/otfabric/go-canopen/store"
)

// TestFDSDO reads and writes a CANopen FD node over USDO.
func TestFDSDO(t *testing.T) {
	fresh(t)
	d, _ := builtinDictionary(5, false)
	n := fdNode(t, d)
	fdNode(t, edsDict(t, "io401.eds", 6))
	fd := []string{"-i", "virtual", "--fd"}
	want(t, run(t, "", append([]string{"sdo", "read", "5", "0x1008", "0"}, fd...)...), 0, "canopenctl simulated node")
	want(t, run(t, "", append([]string{"sdo", "read", "5", "0x1018", "1", "--format", "json", "--client-id", "100"}, fd...)...), 0, `"fd": true`, `"value": 3962`)
	data := strings.Repeat("0123456789abcdef", 64)
	in := filepath.Join(t.TempDir(), "blob.bin")
	_ = os.WriteFile(in, []byte(data), 0o644)
	want(t, run(t, "", append([]string{"sdo", "write", "5", "0x2003", "0", "--file", in, "--block", "--format", "json"}, fd...)...), 0, `"block": true`, `"size": 1024`)
	want(t, run(t, "", append([]string{"sdo", "write", "5", "0x2002", "0", strings.Repeat("x", 100), "-t", "string", "--verify"}, fd...)...), 0, "verified")
	if b, _ := n.Dictionary().Get(0x2003, 0); string(b) != data {
		t.Fatalf("%d bytes", len(b))
	}
	out := filepath.Join(t.TempDir(), "back.bin")
	want(t, run(t, "", append([]string{"sdo", "read", "5", "0x2003", "0", "-o", out}, fd...)...), 0, "1024 bytes written")
	// A USDO abort: exit 11, the SDO equivalent and the USDO code.
	r := run(t, "", append([]string{"sdo", "read", "5", "0x5FFF", "0", "--format", "json"}, fd...)...)
	want(t, r, 11, `"code": "0x06020000"`, `"usdoCode": "0x`, `"remote": true`)
	validateOutput(t, r)
	want(t, run(t, "", append([]string{"sdo", "write", "5", "0x1000", "0", "1", "-t", "u32"}, fd...)...), 11, "")
	want(t, run(t, "", append([]string{"sdo", "read", "9", "0x1000", "0", "--timeout", "30ms", "--retries", "1"}, fd...)...), 5, "")
	want(t, run(t, "", append([]string{"sdo", "dump", "6", "--eds", "testdata/io401.eds", "--range", "1000-1018"}, fd...)...), 0, "Vendor-ID")
	// A classical client gets no answer from an FD node.
	want(t, run(t, "", "sdo", "read", "5", "0x1000", "0", "-i", "virtual", "--timeout", "30ms"), 5, "")
	want(t, run(t, "", "sdo", "read", "5", "0x1000", "0", "-i", "virtual", "--brs"), 3, "--brs needs --fd")
	want(t, run(t, "", "sdo", "read", "5", "0x1000", "0", "-i", "virtual", "--fd", "--client-id", "0"), 0, "")
	want(t, run(t, "", "sdo", "read", "5", "0x1000", "0", "-i", "virtual", "--fd", "--client-id", "200"), 3, "--client-id")
}

// TestFDNetwork runs the network commands against CANopen FD nodes.
func TestFDNetwork(t *testing.T) {
	fresh(t)
	d5, _ := builtinDictionary(5, false)
	_ = d5.SetUint(od.IndexProducerHeartbeat, 0, 10)
	_ = store.AddObjects(d5)
	n := fdNode(t, d5, node.WithStore(store.NewMemory()), node.WithProgram(program.Config{Handler: &memProgram{}}))
	fd := []string{"-i", "virtual", "--fd", "--brs"}
	want(t, run(t, "", append([]string{"scan", "--nodes", "4-6"}, fd...)...), 0, "node   5", "1 of 3 node-IDs answered")
	want(t, run(t, "", append([]string{"heartbeat", "watch", "--count", "1", "--duration", "5s"}, fd...)...), 0, "node 5")
	want(t, run(t, "", append([]string{"nmt", "preop", "5", "--wait"}, fd...)...), 0, "pre-operational")
	want(t, run(t, "", append([]string{"pdo", "show", "5", "--tpdo", "1"}, fd...)...), 0, "TPDO1")
	want(t, run(t, "", append([]string{"pdo", "map", "5", "--tpdo", "2", "--map", "2000:00:32", "--type", "255"}, fd...)...), 0, "TPDO2")
	want(t, run(t, "", append([]string{"mpdo", "configure", "5", "--tpdo", "3", "--mode", "dam", "--cob-id", "385"}, fd...)...), 0, "MPDO")
	cfg := writeConcise(t, cdcf.Entry{Index: simText, Data: []byte("fd")})
	want(t, run(t, "", append([]string{"cdcf", "apply", "5", cfg}, fd...)...), 0, "1 entries")
	want(t, run(t, "", append([]string{"dcf", "apply", "5", "testdata/node37.dcf"}, fd...)...), 11, "")
	want(t, run(t, "", append([]string{"store", "5"}, fd...)...), 0, "saved")
	want(t, run(t, "", append([]string{"restore", "5", "--group", "manufacturer"}, fd...)...), 0, "restores")
	fw := filepath.Join(t.TempDir(), "fw.bin")
	_ = os.WriteFile(fw, []byte(strings.Repeat("firmware", 200)), 0o644)
	want(t, run(t, "", append([]string{"program", "download", "5", fw, "--crc32", "--poll", "5ms"}, fd...)...), 0, "1600 bytes")
	want(t, run(t, "", append([]string{"program", "status", "5"}, fd...)...), 0, "started")
	want(t, run(t, "", append([]string{"sync", "--count", "2", "--period", "1ms"}, fd...)...), 0, "2 SYNC")
	want(t, run(t, "", append([]string{"boot", "--slave", "5=" + cfg, "--no-start", "--master-id", "100"}, fd...)...), 0, "1 of 1 slaves booted")
	// EMCY watch decodes the 20-byte FD emergency.
	r := runWith(t, func() {
		_ = n.EMCY().Raise(context.Background(), 0x6100, emcy.RegGeneric, [5]byte{9})
	}, "emcy", "watch", "--count", "1", "--duration", "5s", "-i", "virtual", "--fd")
	want(t, r, 0, "EMCY 0x6100", "(FD: logical device 0, CiA 301")
	r = runWith(t, func() {
		_ = n.EMCY().Clear(context.Background(), 0x6100)
	}, "emcy", "watch", "--count", "1", "--duration", "5s", "-i", "virtual", "--fd", "--format", "json")
	want(t, r, 0, `"fd":{"logicalDevice":0`)
	validateOutput(t, r)
	want(t, run(t, "", "emcy", "watch", "-i", "virtual", "--brs"), 3, "--brs")
	want(t, run(t, "", "heartbeat", "watch", "-i", "virtual", "--brs"), 3, "--brs")
	want(t, run(t, "", "nmt", "start", "5", "-i", "virtual", "--brs"), 3, "--brs")
	want(t, run(t, "", "scan", "-i", "virtual", "--brs"), 3, "--brs")
	want(t, run(t, "", "sync", "-i", "virtual", "--brs"), 3, "--brs")
	want(t, run(t, "", "pdo", "show", "5", "-i", "virtual", "--brs"), 3, "--brs")
	want(t, run(t, "", "pdo", "map", "5", "--tpdo", "1", "--map", "2000:00:32", "-i", "virtual", "--brs"), 3, "--brs")
	want(t, run(t, "", "dcf", "apply", "5", "testdata/node37.dcf", "-i", "virtual", "--brs"), 3, "--brs")
	want(t, run(t, "", "simulate", "-i", "virtual", "--node-id", "9", "--brs"), 3, "--brs")
}

// TestFDMonitor decodes USDO and FD traffic.
func TestFDMonitor(t *testing.T) {
	fresh(t)
	d, _ := builtinDictionary(5, false)
	n := fdNode(t, d)
	wait := background("monitor", "-i", "virtual", "--fd", "--service", "usdo", "--node", "5", "--count", "2", "--duration", "10s")
	time.Sleep(50 * time.Millisecond)
	want(t, run(t, "", "sdo", "read", "5", "0x1018", "1", "-i", "virtual", "--fd"), 0)
	want(t, wait(), 0, "USDO request node 127", "USDO response node 5")
	r := runWith(t, func() {
		_ = n.EMCY().Raise(context.Background(), 0x5000, emcy.RegGeneric, [5]byte{})
	}, "monitor", "-i", "virtual", "--fd", "--service", "emcy", "--count", "1", "--duration", "5s", "--format", "json")
	want(t, r, 0, `"fd":true`, `"kind":"emcy"`, `"logicalDevice"`)
	validateOutput(t, r)
	r = runWith(t, func() {
		want(t, run(t, "", "sdo", "read", "5", "0x1000", "0", "-i", "virtual", "--fd"), 0)
	}, "monitor", "-i", "virtual", "--fd", "--service", "usdo", "--count", "2", "--duration", "5s", "--format", "json")
	want(t, r, 0, `"kind":"usdo-request"`, `"usdo":{`)
	validateOutput(t, r)
	// Without --fd the monitor receives no FD frames.
	r = runWith(t, func() {
		_ = n.EMCY().Clear(context.Background(), 0x5000)
	}, "monitor", "-i", "virtual", "--service", "emcy", "--duration", "300ms")
	want(t, r, 0, "0 shown")
	_ = canopen.NodeID(0)
}
