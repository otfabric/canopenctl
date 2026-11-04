// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/otfabric/go-canbus"
)

// sdoResponder answers the SDO requests to node 5 (0x605) on the shared
// virtual bus with reply, as a hostile or broken device would; it counts
// the requests it saw.
func sdoResponder(t *testing.T, reply func(req canbus.Frame) (canbus.Frame, bool)) *atomic.Int64 {
	t.Helper()
	b := port(t)
	sub, err := b.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var n atomic.Int64
	go func() {
		defer close(done)
		for {
			f, err := sub.Receive(ctx)
			if err != nil {
				return
			}
			if f.ID != 0x605 {
				continue
			}
			n.Add(1)
			if r, ok := reply(f); ok {
				r.ID, r.Len = 0x585, 8
				_ = b.Send(ctx, &r)
			}
		}
	}()
	t.Cleanup(func() { cancel(); <-done })
	return &n
}

// TestHostileSDOServer: a device that announces 4 GiB, or streams
// segments without end, cannot make canopenctl hold more than --max-size:
// the upload is aborted (exit 6, the message names --max-size) after at
// most --max-size bytes, quickly.
func TestHostileSDOServer(t *testing.T) {
	t.Run("announces 4 GiB", func(t *testing.T) {
		fresh(t)
		n := sdoResponder(t, func(req canbus.Frame) (canbus.Frame, bool) {
			if req.Data[0]&0xE0 != 0x40 { // only the initiate upload
				return canbus.Frame{}, false
			}
			var r canbus.Frame
			r.Data[0] = 0x41 // segmented, size indicated
			copy(r.Data[1:4], req.Data[1:4])
			binary.LittleEndian.PutUint32(r.Data[4:], 0xFFFFFFFF)
			return r, true
		})
		start := time.Now()
		want(t, run(t, "", "sdo", "read", "5", "0x2000", "0", "-i", "virtual", "--timeout", "200ms"), 6, "0x05040005", "larger than --max-size")
		if d := time.Since(start); d > 5*time.Second {
			t.Fatalf("took %v", d)
		}
		if got := n.Load(); got > 2 { // the initiate and the abort
			t.Fatalf("%d requests: segments were requested", got)
		}
	})
	t.Run("endless segments", func(t *testing.T) {
		fresh(t)
		sdoResponder(t, func(req canbus.Frame) (canbus.Frame, bool) {
			var r canbus.Frame
			switch req.Data[0] & 0xE0 {
			case 0x40: // initiate: segmented, no size indicated
				r.Data[0] = 0x40
				copy(r.Data[1:4], req.Data[1:4])
			case 0x60: // upload segment: 7 bytes, never the last
				r.Data[0] = req.Data[0] & 0x10
				copy(r.Data[1:], "AAAAAAA")
			default:
				return canbus.Frame{}, false
			}
			return r, true
		})
		start := time.Now()
		want(t, run(t, "", "sdo", "read", "5", "0x2000", "0", "-i", "virtual", "--timeout", "200ms", "--max-size", "64"), 6, "0x05040005", "larger than --max-size")
		if d := time.Since(start); d > 5*time.Second {
			t.Fatalf("took %v", d)
		}
	})
}

// fuzzRun runs canopenctl with stdin and reports the exit code; a panic
// fails the fuzz target.
func fuzzRun(stdin []byte, args ...string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var out, errOut bytes.Buffer
	return Run(ctx, args, bytes.NewReader(stdin), &out, &errOut)
}

// FuzzDecodeTrace feeds arbitrary bytes to decode as a trace on stdin:
// every format is detected from the content, so the input reaches each
// trace reader. It must end with exit 0 (decoded), 1 (malformed
// messages) or 3 (not a trace), never a panic or another code.
func FuzzDecodeTrace(f *testing.F) {
	f.Add([]byte("(1.000000) vcan0 000#0105\n(1.001000) vcan0 705#7F\n"))
	f.Add([]byte("{\"t\":1,\"id\":1797,\"data\":\"7f\"}\n"))
	f.Add([]byte("\xd4\xc3\xb2\xa1\x02\x00\x04\x00"))         // pcap, truncated
	f.Add([]byte("\x0a\x0d\x0d\x0a\x1c\x00\x00\x00\x4d\x3c")) // pcapng, truncated
	f.Add([]byte("LOGG\x90\x00\x00\x00"))                     // BLF, truncated
	f.Add([]byte("date Mon Nov 4 10:00:00 am 2026\nbase hex timestamps absolute\n"))
	f.Add([]byte(";$FILEVERSION=2.1\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if code := fuzzRun(data, "decode", "-"); code != 0 && code != 1 && code != 3 {
			t.Fatalf("exit %d", code)
		}
	})
}

// FuzzEDSFile feeds arbitrary files to eds validate and eds show: exit 0
// or 1 (validation findings) or 3 (not an EDS), never a panic.
func FuzzEDSFile(f *testing.F) {
	if b, err := os.ReadFile(filepath.Join("testdata", "io401.eds")); err == nil {
		f.Add(b)
		f.Add(b[:len(b)/2])
	}
	f.Add([]byte("[FileInfo]\nFileName=x.eds\n[1000]\nDataType=0x0007\nDefaultValue=$NODEID+0xFFFFFFFF\n"))
	f.Add([]byte("[1F50]\nSubNumber=255\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		path := filepath.Join(t.TempDir(), "f.eds")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"eds", "validate", path}, {"eds", "show", path}} {
			if code := fuzzRun(nil, args...); code != 0 && code != 1 && code != 3 {
				t.Fatalf("%s: exit %d", strings.Join(args, " "), code)
			}
		}
	})
}
