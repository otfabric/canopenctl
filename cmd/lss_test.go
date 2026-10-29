// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/otfabric/go-canopen"
	"github.com/otfabric/go-canopen/lss"
	"github.com/otfabric/go-canopen/node"
	"github.com/otfabric/go-canopen/store"
)

// known restricts Fastscan to the built-in device's vendor, product and
// revision, with a short window.
var known = []string{"--vendor", "0xF7A", "--product", "0xC0C7", "--revision", "0x10000", "--window", "10ms"}

func TestLSSAssignAll(t *testing.T) {
	fresh(t)
	var nodes []*node.Node
	for _, s := range []uint32{0x1001, 0x1002, 0x1003} {
		nodes = append(nodes, lssNode(t, 0, s))
	}
	configured := lssNode(t, 5, 5)
	v := []string{"-i", "virtual"}
	want(t, run(t, "", append([]string{"lss", "identify"}, v...)...), 0, "a device without node-ID answered")
	r := run(t, "", append(append([]string{"lss", "fastscan", "--format", "table"}, known...), v...)...)
	want(t, r, 0, "ADDRESS", lssAddr(0x1001), lssAddr(0x1002), lssAddr(0x1003))
	want(t, run(t, "", append(append([]string{"lss", "fastscan", "--max", "1"}, known...), v...)...), 0, "1 unconfigured devices")
	// An unconfigured device answers inquire with node-ID 255.
	want(t, run(t, "", append([]string{"lss", "inquire", "--address", lssAddr(0x1002)}, v...)...), 0, "unconfigured (no node-ID)")
	r = run(t, "", append(append([]string{"lss", "assign-all", "--first", "4", "--avoid", "5", "--format", "json"}, known...), v...)...)
	want(t, r, 0, `"command": "lss assign-all"`, `"nodeId": 4`, `"nodeId": 6`, `"nodeId": 7`)
	got := map[canopen.NodeID]bool{}
	for _, n := range nodes {
		eventually(t, "node-ID", func() bool { return n.NodeID() != lss.Unconfigured })
		got[n.NodeID()] = true
	}
	if !got[4] || !got[6] || !got[7] {
		t.Fatalf("node-IDs %v", got)
	}
	eventually(t, "boot-up", func() bool { return nodes[0].State() == canopen.StatePreOperational })
	want(t, run(t, "", "sdo", "read", "4", "0x1018", "4", "-i", "virtual"), 0, "0x1018:04")
	// Nothing left: no device answers, assign-all ends at once.
	want(t, run(t, "", append([]string{"lss", "identify"}, v...)...), 0, "no device without node-ID answered")
	want(t, run(t, "", append(append([]string{"lss", "assign-all", "--first", "10"}, known...), v...)...), 0, "0 devices given a node-ID")
	want(t, run(t, "", append(append([]string{"lss", "assign-all", "--first", "10", "--format", "table", "--store"}, known...), v...)...), 0, "NODE  ADDRESS")
	want(t, run(t, "", append(append([]string{"lss", "fastscan"}, known...), v...)...), 0, "0 unconfigured devices")
	_ = configured
}

func TestLSSConfigure(t *testing.T) {
	fresh(t)
	n := lssNode(t, 5, 5)
	mem := store.NewMemory()
	lssNode(t, 6, 6, node.WithStore(mem))
	v := []string{"-i", "virtual", "--timeout", "200ms"}
	a5, a6 := lssAddr(5), lssAddr(6)
	want(t, run(t, "", append([]string{"lss", "inquire", "--address", a5, "--format", "json"}, v...)...), 0, `"nodeId": 5`, `"configured": true`)
	// Select, inquire, configure, back to waiting: a configured device
	// applies the node-ID at its next reset communication.
	want(t, run(t, "", append([]string{"lss", "select", a5}, v...)...), 0, "in the configuration state")
	want(t, run(t, "", append([]string{"lss", "inquire"}, v...)...), 0, "slave "+a5+": node-ID 5")
	want(t, run(t, "", append([]string{"lss", "configure", "--node-id", "20"}, v...)...), 0, "node-ID 20")
	want(t, run(t, "", append([]string{"lss", "switch", "waiting"}, v...)...), 0, "switched to waiting")
	want(t, run(t, "", "nmt", "reset-comm", "5", "-i", "virtual"), 0)
	eventually(t, "node-ID 20", func() bool { return n.NodeID() == 20 })
	// The device refuses a bit rate without a bit timing handler, and a
	// store without a store.
	want(t, run(t, "", append([]string{"lss", "configure", "--address", lssAddr(5), "--bit-rate", "250"}, v...)...), 12, "lss")
	r := run(t, "", append([]string{"lss", "configure", "--address", lssAddr(5), "--store", "--format", "json"}, v...)...)
	want(t, r, 12, `"code": "lss_refused"`)
	r = run(t, "", append([]string{"lss", "configure", "--address", a6, "--node-id", "7", "--store", "--format", "json"}, v...)...)
	want(t, r, 0, `"nodeId": 7`, `"stored": true`, `"address": "`+a6)
	saved, _ := mem.Load()
	found := false
	for _, e := range saved {
		found = found || e.Index == 0 && e.SubIndex == 0 && len(e.Data) == 1 && e.Data[0] == 7
	}
	if !found {
		t.Fatalf("stored %v", saved)
	}
	want(t, run(t, "", append([]string{"lss", "configure", "--address", a6, "--node-id", "none"}, v...)...), 0, "node-ID removed")
	want(t, run(t, "", append([]string{"lss", "switch", "configuration"}, v...)...), 0, "configuration")
	want(t, run(t, "", append([]string{"lss", "activate", "--delay", "10ms", "--format", "json"}, v...)...), 0, `"delayMs": 10`)
	want(t, run(t, "", append([]string{"lss", "switch", "wait"}, v...)...), 0)
	want(t, run(t, "", append([]string{"lss", "identify", "--address", a6}, v...)...), 0, "a device in")
	want(t, run(t, "", append([]string{"lss", "identify", "--address", lssAddr(0), "--to", lssAddr(0xFFFF)}, v...)...), 0, "a device in")
	want(t, run(t, "", append([]string{"lss", "identify", "--address", lssAddr(0x9999)}, v...)...), 0, "no device in")
	// Nobody answers: exit 5.
	r = run(t, "", "lss", "select", "1:2:3:4", "-i", "virtual", "--timeout", "20ms", "--retries", "0", "--format", "json")
	want(t, r, 5, `"code": "no_response"`)
}

func TestLSSInvalid(t *testing.T) {
	fresh(t)
	for _, c := range []struct {
		args []string
		code int
		msg  string
	}{
		{[]string{"switch", "sleeping"}, 3, "unknown LSS state"},
		{[]string{"select", "1:2:3"}, 3, "invalid LSS address"},
		{[]string{"select", "1:2:3:x"}, 3, "invalid LSS address"},
		{[]string{"inquire", "--address", "x"}, 3, "invalid LSS address"},
		{[]string{"configure"}, 3, "--node-id, --bit-rate or --store"},
		{[]string{"configure", "--node-id", "200"}, 3, "invalid node-ID"},
		{[]string{"configure", "--bit-rate", "300"}, 3, "--bit-rate"},
		{[]string{"configure", "--bit-rate", "auto", "--address", "x"}, 3, "invalid LSS address"},
		{[]string{"activate", "--delay", "70s"}, 3, "--delay"},
		{[]string{"identify", "--to", "1:2:3:4"}, 3, "--to needs --address"},
		{[]string{"identify", "--address", "1:2:3:4", "--to", "1:3:3:4"}, 3, "invalid range"},
		{[]string{"identify", "--address", "1:2:3:4", "--to", "x"}, 3, "invalid LSS address"},
		{[]string{"identify", "--address", "x"}, 3, "invalid LSS address"},
		{[]string{"fastscan", "--vendor", "x"}, 3, "invalid value"},
		{[]string{"fastscan", "--max", "-1"}, 3, "--max"},
		{[]string{"assign-all"}, 2, "first"},
		{[]string{"assign-all", "--first", "200"}, 3, "--first"},
		{[]string{"assign-all", "--first", "1", "--max", "-1"}, 3, "--max"},
		{[]string{"assign-all", "--first", "1", "--avoid", "x"}, 3, "node"},
		{[]string{"assign-all", "--first", "1", "--product", "x"}, 3, "invalid value"},
		{[]string{"switch", "waiting", "--timeout", "0s"}, 3, "--timeout"},
		{[]string{"switch", "waiting", "--retries", "11"}, 3, "--retries"},
		{[]string{"switch", "waiting", "--window", "0s"}, 3, "--window"},
		{[]string{"switch", "waiting", "-i", "slcan:/dev/null"}, 9, "--yes"},
		{[]string{"switch", "waiting", "-i", ""}, 2, "no interface"},
		{[]string{"switch", "waiting", "-i", "slcan:/nonexistent/tty", "--yes"}, 4, ""},
	} {
		args := append([]string{"lss"}, c.args...)
		if !contains(c.args, "-i") {
			args = append(args, "-i", "virtual")
		}
		want(t, run(t, "", args...), c.code, c.msg)
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
