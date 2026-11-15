// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/otfabric/go-canbus"
)

// stalledOut is a stdout that holds every write until release is closed:
// an output that does not keep up with the stream.
type stalledOut struct {
	release chan struct{}
	out     *syncBuffer
}

func (s *stalledOut) Write(p []byte) (int, error) {
	<-s.release
	return s.out.Write(p)
}

// sendFunc sends a CAN frame on the virtual bus.
type sendFunc func(id canbus.ID, data ...byte)

// stalledStream runs a command in a synctest bubble, on a virtual bus of
// its own, with a stdout that holds the first write. Once the command
// listens, produce sends frames from another port; then fake time
// advances by settle, after (when not nil) sends more, the output is
// released and the command runs to its end (--duration).
func stalledStream(t *testing.T, args []string, produce func(send sendFunc), settle time.Duration, after func(send sendFunc)) result {
	t.Helper()
	fresh(t)
	var r result
	synctest.Test(t, func(t *testing.T) {
		defer resetVirtual() // the bubble's bus: its goroutines end with the test
		out := &stalledOut{release: make(chan struct{}), out: newSyncBuffer()}
		errOut := newSyncBuffer()
		ctx := context.Background()
		done := make(chan int, 1)
		go func() { done <- Run(ctx, args, strings.NewReader(""), out, errOut) }()
		synctest.Wait() // the command listens (or waits for the output)
		p := port(t)
		n := 0
		send := func(id canbus.ID, data ...byte) {
			f, err := canbus.NewStandard(id, data)
			if err != nil {
				t.Fatal(err)
			}
			if err := p.Send(ctx, &f); err != nil {
				t.Fatal(err)
			}
			// Every frame reaches the command's handler before the next
			// ones: no queue of the bus or of the library overflows.
			if n++; n%16 == 0 {
				synctest.Wait()
			}
		}
		produce(send)
		synctest.Wait()
		time.Sleep(settle)
		synctest.Wait()
		if after != nil {
			after(send)
			synctest.Wait()
		}
		close(out.release)
		r.code = <-done
		r.out, r.errOut = out.out.String(), errOut.String()
	})
	return r
}

// summaryOf decodes the summary record, the last line of a JSON stream.
func summaryOf(t *testing.T, r result) map[string]any {
	t.Helper()
	last := lastLine(r.out)
	var m map[string]any
	if err := json.Unmarshal([]byte(last), &m); err != nil || m["type"] != "summary" {
		t.Fatalf("no summary record (%v): %s", err, last)
	}
	return m
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return lines[len(lines)-1]
}

// count is a number field of a record; -1 when it is missing.
func count(m map[string]any, field string) int {
	v, ok := m[field].(float64)
	if !ok {
		return -1
	}
	return int(v)
}

// records counts the stream records of a type.
func records(out, typ string) int {
	return strings.Count(out, `"type":"`+typ+`"`)
}

// TestHeartbeatWatchDropped: with the output stalled, the boot-ups of node
// 5 fill the queue, and the timeout of the supervised node 6 finds it
// full. The watch exits 1 all the same, whether node 6 is still lost at
// the end or was heard again, and the summary counts what was dropped.
// In fake time node 6 is heard at 0, lost at 100 ms, heard again at 1 s
// (resumed) and the watch ends at 1.05 s, before a second timeout.
func TestHeartbeatWatchDropped(t *testing.T) {
	for _, tc := range []struct {
		name    string
		resumed bool
		format  string
	}{
		{"lost at the end", false, "json"},
		{"resumed before the end", true, "json"},
		{"text output", true, "text"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var after func(sendFunc)
			if tc.resumed {
				after = func(send sendFunc) { send(0x706, 0x7F) } // node 6 is heard again
			}
			r := stalledStream(t, []string{"heartbeat", "watch", "-i", "virtual", "--supervise", "100ms", "--duration", "1050ms", "--format", tc.format},
				func(send sendFunc) {
					send(0x706, 0x7F) // node 6 pre-operational: its record waits for the output
					for range 2 * streamQueue {
						send(0x705, 0x00) // boot-ups of node 5
					}
				}, time.Second, after) // node 6 is silent for 1 s: lost
			if r.code != 1 {
				t.Fatalf("exit %d, want 1: node 6 was lost\nstderr: %s\nlast record: %s", r.code, r.errOut, lastLine(r.out))
			}
			// One record waited for the output and the queue held 1024: the
			// other boot-ups and the timeout were dropped, and when node 6
			// was heard again, its resumption and state change too.
			wantDropped := 2*streamQueue - streamQueue + 1
			if tc.resumed {
				wantDropped += 2
			}
			if tc.format == "text" {
				if !strings.Contains(r.errOut, fmt.Sprintf("%d events dropped: the output did not keep up", wantDropped)) {
					t.Fatalf("no line about the dropped events:\n%s", r.errOut)
				}
				return
			}
			s := summaryOf(t, r)
			if d := count(s, "dropped"); d != wantDropped {
				t.Fatalf("summary dropped = %d, want %d: %v", d, wantDropped, s)
			}
			if n := count(s, "timeouts"); n != 0 || records(r.out, "heartbeat") != streamQueue+1 {
				t.Fatalf("timeouts = %d, %d heartbeat records; want 0 and %d: %v", n, records(r.out, "heartbeat"), streamQueue+1, s)
			}
		})
	}
}

// TestStreamsDropped: emcy watch, pdo watch and simulate count what they
// drop when the output does not keep up.
func TestStreamsDropped(t *testing.T) {
	const sent = 2 * streamQueue
	for _, tc := range []struct {
		name    string
		args    []string
		frame   func(send sendFunc, i int)
		typ     string
		queue   int // records the queue holds
		taken   int // records taken from the queue before the output stalled
		wantMsg string
	}{
		{"emcy watch", []string{"emcy", "watch"}, func(send sendFunc, i int) { send(0x085, 0x10, 0x23, 0x02, 1, 2, 3, 0, byte(i)) },
			"emcy", streamQueue, 1, "emergency messages dropped"},
		{"pdo watch", []string{"pdo", "watch", "5", "--tpdo", "1", "--cob-id", "185", "--map", "6000:01:8"}, func(send sendFunc, i int) { send(0x185, byte(i)) },
			"pdo", streamQueue, 1, "PDOs dropped"},
		// simulate's first write is its ready record: nothing is taken
		// from the queue of state changes before the output stalls. NMT
		// start and stop, alternating, change the state every time.
		{"simulate", []string{"simulate", "--node-id", "5"}, func(send sendFunc, i int) { send(0x000, byte(1+i%2), 5) },
			"state", simStateQueue, 0, "state changes dropped"},
	} {
		for _, format := range []string{"json", "text"} {
			t.Run(tc.name+" "+format, func(t *testing.T) {
				args := append(append([]string{}, tc.args...), "-i", "virtual", "--duration", "10s", "--format", format)
				r := stalledStream(t, args, func(send sendFunc) {
					for i := range sent {
						tc.frame(send, i)
					}
				}, 0, nil)
				if r.code != 0 {
					t.Fatalf("exit %d\nstderr: %s\nlast record: %s", r.code, r.errOut, lastLine(r.out))
				}
				if format == "text" {
					if !strings.Contains(r.errOut, tc.wantMsg+": the output did not keep up") {
						t.Fatalf("no line about the dropped records:\n%s", r.errOut)
					}
					return
				}
				s := summaryOf(t, r)
				n := records(r.out, tc.typ)
				d := count(s, "dropped")
				if d <= 0 || n != tc.queue+tc.taken {
					t.Fatalf("%d %s records, dropped %d; want %d records and the rest dropped: %v", n, tc.typ, d, tc.queue+tc.taken, s)
				}
				if tc.typ != "state" && n+d != sent {
					t.Fatalf("%d records + %d dropped, want %d frames in all", n, d, sent)
				}
			})
		}
	}
}
