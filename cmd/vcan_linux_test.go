// SPDX-License-Identifier: MIT

//go:build linux

package cmd

import (
	"net"
	"os"
	"testing"

	"github.com/otfabric/go-canbus"
	"github.com/otfabric/go-canbus/socketcan"
	"github.com/otfabric/go-canopen"
	"github.com/otfabric/go-canopen/node"
	"github.com/otfabric/go-canopen/od"
)

// vcan0 returns $CANOPENCTL_TEST_IFACE (default vcan0), a vcan interface
// that is up; missing, it fails with CANOPENCTL_REQUIRE_VCAN=1 and skips
// loudly otherwise.
func vcan0(t *testing.T) string {
	t.Helper()
	name := os.Getenv("CANOPENCTL_TEST_IFACE")
	if name == "" {
		name = "vcan0"
	}
	ifi, err := net.InterfaceByName(name)
	if err != nil || ifi.Flags&net.FlagUp == 0 {
		if os.Getenv("CANOPENCTL_REQUIRE_VCAN") == "1" {
			t.Fatalf("no usable %s (CANOPENCTL_REQUIRE_VCAN=1)", name)
		}
		t.Skipf("SKIP: no usable %s: sudo ip link add %s type vcan && sudo ip link set %s up", name, name, name)
	}
	return name
}

// vcanBus opens a socket on a vcan interface, closed with the test.
func vcanBus(t *testing.T, ifc string) *canbus.Bus {
	t.Helper()
	s, err := socketcan.Open(ifc)
	if err != nil {
		t.Fatal(err)
	}
	b, err := canbus.NewBus(s, canbus.WithInterfaceName(ifc))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}

// TestVCANCommands drives a node on vcan0 with the commands on their own
// sockets: on vcan no --yes is needed.
func TestVCANCommands(t *testing.T) {
	ifc := vcan0(t)
	d := edsDict(t, "io401.eds", 42)
	n, err := newNodeOn(vcanBus(t, ifc), d)
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()
	_ = n.Dictionary().SetUint(od.IndexProducerHeartbeat, 0, 20)
	want(t, run(t, "", "sdo", "read", "42", "0x1018", "1", "-i", ifc), 0, "3962")
	want(t, run(t, "", "sdo", "write", "42", "0x6200", "1", "0x5A", "--eds", "testdata/io401.eds", "-i", ifc, "--block"), 0, "written")
	want(t, run(t, "", "nmt", "start", "42", "-i", ifc, "--wait"), 0, "operational")
	want(t, run(t, "", "scan", "-i", ifc, "--nodes", "40-44", "--timeout", "50ms"), 0, "node  42")
	want(t, run(t, "", "pdo", "show", "42", "-i", ifc, "--tpdo", "1"), 0, "TPDO1")
	want(t, run(t, "", "heartbeat", "watch", "-i", ifc, "--node", "42", "--supervise", "500ms", "--duration", "100ms"), 0, "node 42: operational")
	want(t, run(t, "", "sync", "-i", ifc, "--count", "3", "--period", "1ms"), 0, "3 SYNC")
	want(t, run(t, "", "time", "-i", ifc), 0, "TIME")
	want(t, run(t, "", "monitor", "-i", ifc, "--node", "42", "--service", "heartbeat", "--count", "1"), 0, "heartbeat node 42")
	// can0-like interfaces that do not exist: transport error, not safety.
	want(t, run(t, "", "nmt", "start", "all", "-i", "nosuchcan0"), 4, "")
}

// newNodeOn starts a node with a dictionary on a bus; the caller closes it.
func newNodeOn(b canopen.Bus, d *od.Dictionary) (*node.Node, error) {
	n, err := node.New(b, d)
	if err != nil {
		return nil, err
	}
	if err := n.Start(); err != nil {
		return nil, err
	}
	return n, nil
}
