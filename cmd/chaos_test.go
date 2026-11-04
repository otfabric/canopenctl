// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/otfabric/go-canbus"
	"github.com/otfabric/go-canbus/chaos"
	"github.com/otfabric/go-canbus/virtual"
	"github.com/otfabric/go-canopen/node"
)

// chaosNode starts node 5 (io401.eds) on its own virtual bus, joined to
// canopenctl's virtual bus by a chaos bridge with cfg: every frame
// between them crosses the faults.
func chaosNode(t *testing.T, cfg chaos.Config) *node.Node {
	t.Helper()
	dev := virtual.New(virtual.Config{Name: "dev"})
	t.Cleanup(func() { _ = dev.Close() })
	devSide, err := dev.Open()
	if err != nil {
		t.Fatal(err)
	}
	ctlSide, err := sharedVirtual().Open()
	if err != nil {
		t.Fatal(err)
	}
	br, err := chaos.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); _ = br.Both(ctx, devSide, ctlSide) }()
	nodePort, err := dev.Open()
	if err != nil {
		t.Fatal(err)
	}
	b, err := canbus.NewBus(nodePort)
	if err != nil {
		t.Fatal(err)
	}
	n, err := node.New(b, edsDict(t, "io401.eds", 5))
	if err != nil {
		t.Fatal(err)
	}
	if err := n.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = n.Close()
		_ = b.Close()
		cancel()
		_ = ctlSide.Close()
		_ = devSide.Close()
		<-done
	})
	return n
}

// TestMonkeySDO: sdo read (segmented and block) and sdo write --verify
// through a bus that drops, duplicates, delays and reorders frames.
// canopenctl either reports the right data with exit 0 or fails with a
// timeout, protocol or abort exit code; it never reports wrong data.
// --retries repeats timeouts and transfers broken by a lost, duplicated
// or reordered frame (toggle, command, block aborts), so nearly every run
// succeeds; before, those aborts ended about half of the runs. Corruption is left out: classical SDO segments carry no CRC
// (go-canopen LIMITATIONS.md).
func TestMonkeySDO(t *testing.T) {
	const name = "OTF IO-401 8DI/8DO 2AI/2AO"
	ok, runs := 0, 0
	for seed := int64(1); seed <= 8; seed++ {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			fresh(t)
			n := chaosNode(t, chaos.Config{Seed: seed, Drop: 0.05, Duplicate: 0.01, Delay: 0.05, Reorder: 0.05})
			common := []string{"-i", "virtual", "--timeout", "100ms", "--retries", "5"}
			for _, args := range [][]string{
				{"sdo", "read", "5", "0x1008", "0", "--type", "string"},
				{"sdo", "read", "5", "0x1008", "0", "--type", "string", "--block"},
			} {
				r := run(t, "", append(args, common...)...)
				runs++
				switch r.code {
				case 0:
					if !strings.Contains(r.out, name) {
						t.Fatalf("%v: wrong data reported: %s", args, r.out)
					}
					ok++
				case 5, 6, 11:
				default:
					t.Fatalf("%v: exit %d\n%s%s", args, r.code, r.out, r.errOut)
				}
			}
			value := fmt.Sprint(100 + seed)
			r := run(t, "", append([]string{"sdo", "write", "5", "0x1017", "0", value, "--type", "u16", "--verify"}, common...)...)
			runs++
			switch r.code {
			case 0:
				if v, _ := n.Dictionary().GetUint(0x1017, 0); fmt.Sprint(v) != value {
					t.Fatalf("write reported verified, device has %d", v)
				}
				ok++
			case 5, 6, 11:
			default:
				t.Fatalf("write: exit %d\n%s%s", r.code, r.out, r.errOut)
			}
		})
	}
	if ok*10 < runs*9 {
		t.Fatalf("%d of %d runs succeeded with --retries 5", ok, runs)
	}
	t.Logf("%d of %d runs succeeded", ok, runs)
}
