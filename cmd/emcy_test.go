// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/otfabric/go-canopen/emcy"
	"github.com/otfabric/go-canopen/node"
	syncobj "github.com/otfabric/go-canopen/sync"
	"github.com/otfabric/go-canopen/timestamp"
)

// raiseUntil raises and clears an emergency on a node every 10 ms until
// the returned stop is called.
func raiseUntil(n *node.Node, code uint16) (stop func()) {
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-time.After(10 * time.Millisecond):
			}
			_ = n.EMCY().Raise(context.Background(), code, emcy.RegCurrent, [5]byte{1, 2, 3})
			_ = n.EMCY().Clear(context.Background(), code)
		}
	}()
	return func() { close(done) }
}

func TestEMCYWatch(t *testing.T) {
	fresh(t)
	n := ioNode(t, 5)
	m := ioNode(t, 6)
	stop := raiseUntil(n, 0x2310)
	stop2 := raiseUntil(m, 0x4210)
	r := run(t, "", "emcy", "watch", "-i", "virtual", "--node", "5", "--count", "4", "--format", "json")
	want(t, r, 0, `"type":"emcy"`, `"code":"0x2310"`, `"codeText":"current, device output side (0x2310)"`, `"msef":"0102030000"`, `"reset":true`, `"type":"summary"`)
	if strings.Contains(r.out, "0x4210") {
		t.Fatal("node filter")
	}
	r = run(t, "", "emcy", "watch", "-i", "virtual", "--count", "2")
	stop()
	stop2()
	want(t, r, 0, "EMCY 0x")
	want(t, run(t, "", "emcy", "watch", "-i", "virtual", "--duration", "20ms"), 0, "emergency messages")
	want(t, run(t, "", "emcy", "watch", "-i", "virtual", "--node", "0"), 3, "invalid")
}

func TestSync(t *testing.T) {
	fresh(t)
	var got atomic.Int64
	var last atomic.Int64
	c, err := syncobj.NewConsumer(port(t), syncobj.WithHandler(func(e syncobj.Event) { got.Add(1); last.Store(int64(e.Counter)) }))
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Start()
	defer c.Close()
	want(t, run(t, "", "sync", "-i", "virtual", "--count", "5", "--period", "2ms", "--counter", "3"), 0, "5 SYNC sent on 0x080", "counter 1..3")
	eventually(t, "5 SYNC", func() bool { return got.Load() == 5 })
	if last.Load() != 2 {
		t.Fatalf("last counter %d", last.Load())
	}
	want(t, run(t, "", "sync", "-i", "virtual", "--count", "0", "--duration", "30ms", "--period", "5ms", "--format", "json"), 0, `"command": "sync"`, `"periodUs": 5000`)
	want(t, run(t, "", "sync", "-i", "virtual", "--count", "0"), 3, "--duration")
	want(t, run(t, "", "sync", "-i", "virtual", "--period", "10us"), 3, "--period")
	want(t, run(t, "", "sync", "-i", "virtual", "--counter", "1"), 3, "--counter")
	want(t, run(t, "", "sync", "-i", "virtual", "--cob-id", "zz"), 3, "COB-ID")
	want(t, run(t, "", "sync", "-i", "virtual", "--count", "-1"), 3, "--count")
	want(t, run(t, "", "sync", "-i", "slcan:/dev/null"), 9, "--yes")
}

func TestTime(t *testing.T) {
	fresh(t)
	var got atomic.Value
	c, err := timestamp.NewConsumer(port(t), timestamp.WithHandler(func(e timestamp.Event) { got.Store(e.Time) }))
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Start()
	defer c.Close()
	want(t, run(t, "", "time", "-i", "virtual", "--at", "2026-10-29T12:00:00Z"), 0, "1 TIME message(s) sent on 0x100: 2026-10-29T12:00:00Z")
	eventually(t, "TIME", func() bool {
		v, _ := got.Load().(time.Time)
		return v.Equal(time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC))
	})
	want(t, run(t, "", "time", "-i", "virtual", "--count", "2", "--period", "1ms", "--format", "json"), 0, `"sent": 2`)
	want(t, run(t, "", "time", "-i", "virtual", "--at", "yesterday"), 3, "--at")
	want(t, run(t, "", "time", "-i", "virtual", "--at", "1970-01-01T00:00:00Z"), 3, "--at")
	want(t, run(t, "", "time", "-i", "virtual", "--cob-id", "x"), 3, "COB-ID")
	want(t, run(t, "", "time", "-i", "virtual", "--count", "0"), 3, "--duration")
	want(t, run(t, "", "time", "-i", "slcan:/dev/null"), 9, "--yes")
}
