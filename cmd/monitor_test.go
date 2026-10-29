// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/otfabric/go-canbus"
	"github.com/otfabric/go-canbus/capture"
	"github.com/otfabric/go-canopen/od"
)

// traffic sends frames on the virtual bus repeatedly until stop.
func traffic(t *testing.T, frames []canbus.Frame) (stop func()) {
	t.Helper()
	b := port(t)
	done := make(chan struct{})
	go func() {
		for {
			for i := range frames {
				select {
				case <-done:
					return
				default:
				}
				_ = b.Send(context.Background(), &frames[i])
				time.Sleep(time.Millisecond)
			}
		}
	}()
	return func() { close(done) }
}

func frame(s string) canbus.Frame {
	var f canbus.Frame
	if err := capture.ParseFrame(s, &f); err != nil {
		panic(err)
	}
	return f
}

func TestMonitor(t *testing.T) {
	fresh(t)
	stop := traffic(t, []canbus.Frame{frame("000#0105"), frame("080#"), frame("085#1023010102030000"), frame("705#7F"), frame("605#4018100100000000"),
		frame("585#43181001EF0F0000"), frame("185#0102"), frame("701#05"), frame("080#0102")})
	defer stop()
	r := run(t, "", "monitor", "-i", "virtual", "--node", "5", "--count", "8", "--format", "json")
	want(t, r, 0, `"kind":"nmt"`, `"kind":"emcy"`, `"codeText":"current, device output side (0x2310)"`, `"kind":"heartbeat"`, `"kind":"sdo-request"`, `"kind":"tpdo"`, `"type":"summary"`)
	if strings.Contains(r.out, `"node":1,`) {
		t.Fatal("node filter")
	}
	r = run(t, "", "monitor", "-i", "virtual", "--service", "sdo", "--count", "2")
	want(t, r, 0, "SDO", "2 shown")
	r = run(t, "", "monitor", "-i", "virtual", "--service", "sync", "-n", "3")
	want(t, r, 0, "malformed")
	want(t, run(t, "", "monitor", "-i", "virtual", "--duration", "20ms", "--service", "heartbeat,lss"), 0, "shown")
	want(t, run(t, "", "monitor", "-i", "virtual", "--service", "bogus"), 3, "unknown service")
	want(t, run(t, "", "monitor", "-i", "virtual", "--count", "-1"), 3, "--count")
	want(t, run(t, "", "monitor", "-i", "virtual", "--node", "1-x"), 3, "invalid")
	want(t, run(t, "", "monitor"), 2, "no interface")
}

func TestMonitorNodeTraffic(t *testing.T) {
	fresh(t)
	n := ioNode(t, 5)
	_ = n.Dictionary().SetUint(od.IndexProducerHeartbeat, 0, 5)
	r := run(t, "", "monitor", "-i", "virtual", "--count", "2", "--service", "heartbeat")
	want(t, r, 0, "heartbeat node 5")
}

const trace = `(1700000000.000000) vcan0 000#0105
(1700000000.001000) vcan0 705#00
(1700000000.002000) vcan0 605#4008100000000000
(1700000000.003000) vcan0 585#4108100006000000
(1700000000.004000) vcan0 605#6000000000000000
(1700000000.005000) vcan0 585#00494F2D3430310D
(1700000000.006000) vcan0 085#00
(1700000000.007000) vcan0 100#E8030000A714
(1700000000.008000) vcan0 7E5#4C00000000000000
(1700000000.009000) vcan0 705#R
(1700000000.010000) vcan0 1F5#
`

func TestDecode(t *testing.T) {
	fresh(t)
	path := filepath.Join(t.TempDir(), "t.log")
	_ = os.WriteFile(path, []byte(trace), 0o644)
	r := run(t, "", "decode", path)
	want(t, r, 1, "NMT start node 5", "boot-up", "initiate upload 0x1008:00", "upload segment", "malformed")
	golden(t, "decode.txt", r.out)
	r = run(t, "", "decode", path, "--format", "json", "--ignore-malformed")
	want(t, r, 0, `"kind":"time"`, `"kind":"lss-master"`, `"kind":"guard-request"`)
	golden(t, "decode.jsonl", r.out)
	want(t, run(t, trace, "decode", "-", "--node", "5", "--service", "sdo"), 1, "4 shown")
	want(t, run(t, "", "decode", filepath.Join(t.TempDir(), "missing.log")), 3, "cannot open trace")
	bad := filepath.Join(t.TempDir(), "bad.log")
	_ = os.WriteFile(bad, []byte("this is not a trace\n"), 0o644)
	want(t, run(t, "", "decode", bad), 3, "")
	want(t, run(t, "", "decode", path, "--service", "x"), 3, "unknown service")
}
