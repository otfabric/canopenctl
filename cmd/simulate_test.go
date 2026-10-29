// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"strings"
	"testing"
	"time"
)

// simulateWith runs simulate in the background on the virtual bus and
// runs probe against it once the node answers SDO.
func simulateWith(t *testing.T, simArgs []string, probe func()) result {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan result, 1)
	go func() { done <- runCtx(ctx, append([]string{"simulate", "-i", "virtual"}, simArgs...)...) }()
	time.Sleep(50 * time.Millisecond)
	probe()
	cancel()
	select {
	case r := <-done:
		return r
	case <-time.After(10 * time.Second):
		t.Fatal("simulate did not end")
	}
	return result{}
}

func TestSimulateBuiltin(t *testing.T) {
	fresh(t)
	r := simulateWith(t, []string{"--node-id", "5", "--heartbeat", "10", "--tick", "2000:00", "--tick-period", "2ms", "--emcy", "5000:81:0102", "--format", "json"}, func() {
		want(t, run(t, "", "sdo", "read", "5", "0x1008", "0", "-i", "virtual"), 0, "canopenctl simulated node")
		want(t, run(t, "", "emcy", "watch", "-i", "virtual", "--count", "1", "--duration", "2s"), 0, "")
		want(t, run(t, "", "nmt", "start", "5", "-i", "virtual", "--wait"), 0, "operational")
		want(t, run(t, "", "pdo", "watch", "5", "--tpdo", "1", "-i", "virtual", "--count", "2"), 0, "TPDO1 185#")
	})
	want(t, r, 0, `"type":"ready"`, `"type":"state"`, `"command":"start"`, `"type":"summary"`, `"stats"`)
}

func TestSimulateEDSDrive(t *testing.T) {
	fresh(t)
	r := simulateWith(t, []string{"--eds", "testdata/drive402.eds", "--node-id", "3", "--drive", "--auto-start"}, func() {
		want(t, run(t, "", "cia402", "enable", "3", "-i", "virtual", "--poll", "1ms"), 0, "Operation enabled")
	})
	want(t, r, 0, "node 3 running (testdata/drive402.eds", "-> operational")
	if !strings.Contains(r.errOut, "node 3:") {
		t.Fatal(r.errOut)
	}
	// A DCF gives the node-ID.
	fresh(t)
	r = simulateWith(t, []string{"--eds", "testdata/node37.dcf", "--duration", "5s"}, func() {
		want(t, run(t, "", "scan", "-i", "virtual", "--nodes", "37"), 0, "node  37")
	})
	want(t, r, 0, "node 37 running")
	want(t, run(t, "", "simulate", "-i", "virtual", "--node-id", "3", "--duration", "20ms"), 0, "node 3 running")
}

func TestSimulateInvalid(t *testing.T) {
	fresh(t)
	for _, c := range []struct {
		args []string
		msg  string
	}{
		{[]string{"--node-id", "0"}, "--node-id"},
		{[]string{"--eds", "testdata/io401.eds"}, "--node-id"},
		{[]string{"--eds", "testdata/missing.eds", "--node-id", "5"}, "cannot read"},
		{[]string{"--node-id", "5", "--tick", "2000"}, "--tick"},
		{[]string{"--node-id", "5", "--tick", "x:00"}, "invalid index"},
		{[]string{"--node-id", "5", "--tick", "2000:zz"}, "--tick"},
		{[]string{"--node-id", "5", "--tick", "4000:00"}, "no such object"},
		{[]string{"--node-id", "5", "--tick", "2002:00"}, "not an unsigned"},
		{[]string{"--node-id", "5", "--emcy", "0"}, "--emcy"},
		{[]string{"--node-id", "5", "--emcy", "1000:zz"}, "register"},
		{[]string{"--node-id", "5", "--emcy", "1000:01:0102030405060708"}, "MSEF"},
		{[]string{"--node-id", "5", "--emcy", "1:2:3:4"}, "--emcy"},
		{[]string{"--node-id", "5", "--heartbeat", "70000"}, "--heartbeat"},
		{[]string{"--node-id", "5", "--tick-period", "0s"}, "--tick-period"},
	} {
		want(t, run(t, "", append([]string{"simulate", "-i", "virtual", "--duration", "10ms"}, c.args...)...), 3, c.msg)
	}
	want(t, run(t, "", "simulate", "-i", "slcan:/dev/null", "--node-id", "5"), 9, "--yes")
	want(t, run(t, "", "simulate", "-i", "virtual", "--eds", "testdata/compact.eds", "--node-id", "5", "--heartbeat", "100", "--duration", "10ms"), 3, "0x1017")
}
