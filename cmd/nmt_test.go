// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/otfabric/go-canopen"
	"github.com/otfabric/go-canopen/nmt"
	"github.com/otfabric/go-canopen/od"
)

func TestNMT(t *testing.T) {
	fresh(t)
	n := ioNode(t, 5)
	_ = n.Dictionary().SetUint(od.IndexProducerHeartbeat, 0, 20)
	want(t, run(t, "", "nmt", "start", "5", "-i", "virtual"), 0, "NMT start sent to node 5")
	eventually(t, "operational", func() bool { return n.State() == canopen.StateOperational })
	want(t, run(t, "", "nmt", "stop", "all", "-i", "virtual", "--format", "json"), 0, `"command": "stop"`, `"node": 0`)
	eventually(t, "stopped", func() bool { return n.State() == canopen.StateStopped })
	want(t, run(t, "", "nmt", "preop", "5", "-i", "virtual", "--wait"), 0, "reports pre-operational")
	want(t, run(t, "", "nmt", "start", "5", "-i", "virtual", "--wait"), 0, "reports operational")
	want(t, run(t, "", "nmt", "reset-comm", "5", "-i", "virtual", "--wait"), 0, "reports boot-up")
	want(t, run(t, "", "nmt", "reset-node", "5", "-i", "virtual", "--wait", "--format", "json"), 0, `"waited": true`)
	want(t, run(t, "", "nmt", "start", "9", "-i", "virtual", "--wait", "--timeout", "50ms"), 5, "did not report")
	want(t, run(t, "", "nmt", "start", "all", "-i", "virtual", "--wait"), 3, "one node")
	want(t, run(t, "", "nmt", "jump", "5", "-i", "virtual"), 3, "unknown NMT command")
	want(t, run(t, "", "nmt", "start", "128", "-i", "virtual"), 3, "invalid node")
	want(t, run(t, "", "nmt", "start", "5", "-i", "virtual", "--timeout", "0s"), 3, "--timeout")
	want(t, run(t, "", "nmt", "start", "5"), 2, "no interface")
	want(t, run(t, "", "nmt", "start"), 2, "accepts 2 arg")
}

func TestHeartbeatWatch(t *testing.T) {
	fresh(t)
	n := ioNode(t, 5)
	_ = n.Dictionary().SetUint(od.IndexProducerHeartbeat, 0, 10)
	wait := background("heartbeat", "watch", "-i", "virtual", "--node", "5", "--supervise", "100ms", "--duration", "600ms", "--format", "json")
	time.Sleep(100 * time.Millisecond)
	_ = n.SetState(canopen.StateOperational)
	time.Sleep(50 * time.Millisecond)
	_ = n.Dictionary().SetUint(od.IndexProducerHeartbeat, 0, 0) // silent: lost
	time.Sleep(200 * time.Millisecond)
	_ = n.Dictionary().SetUint(od.IndexProducerHeartbeat, 0, 10) // resumed
	r := wait()
	want(t, r, 1, `"type":"heartbeat"`, `"event":"state-change"`, `"event":"timeout"`, `"event":"resumed"`, `"type":"summary"`)
	// Text output, and a boot-up.
	fresh(t)
	m := ioNode(t, 6)
	_ = m.Dictionary().SetUint(od.IndexProducerHeartbeat, 0, 10)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan result, 1)
	go func() { done <- runCtx(ctx, "heartbeat", "watch", "-i", "virtual", "--count", "2") }()
	time.Sleep(100 * time.Millisecond)
	_ = m.Reset(context.Background(), nmt.ResetCommunication)
	r = <-done
	cancel()
	want(t, r, 0, "node 6 boot-up", "2 events, 1 boot-ups, 0 timeouts")
	want(t, run(t, "", "heartbeat", "watch", "-i", "virtual", "--duration", "30ms", "--node", "x"), 3, "invalid")
	want(t, run(t, "", "heartbeat", "watch", "-i", "virtual", "--supervise", "-1s"), 3, "--supervise")
	want(t, run(t, "", "heartbeat", "watch", "-i", "virtual", "--duration", "30ms", "--node", "7"), 0, "0 events")
}

func TestScan(t *testing.T) {
	fresh(t)
	ioNode(t, 5)
	builtinNode(t, 9, true)
	r := run(t, "", "scan", "-i", "virtual", "--nodes", "1-16", "--timeout", "30ms")
	want(t, r, 0, "node   5  device type 0x000F0191 (CiA 401 generic I/O)", "node   9", "CiA 402", "2 of 16 node-IDs answered")
	r = run(t, "", "scan", "-i", "virtual", "--nodes", "5,9", "--format", "json")
	want(t, r, 0, `"vendorId": "0x00000F7A"`, `"scanned": 2`)
	want(t, run(t, "", "scan", "-i", "virtual", "--nodes", "5", "--format", "table"), 0, "NODE", "DEVICE-TYPE")
	want(t, run(t, "", "scan", "-i", "virtual", "--nodes", "0"), 3, "invalid")
	want(t, run(t, "", "scan", "-i", "virtual", "--timeout", "0s"), 3, "--timeout")
	want(t, run(t, "", "scan", "-i", "virtual", "--parallel", "0"), 3, "--parallel")
	if profileName(0) != "none" || profileName(0x1234) != "CiA 4660" {
		t.Fatal(profileName(0x1234))
	}
}
