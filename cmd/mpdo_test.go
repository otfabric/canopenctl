// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/otfabric/go-canbus"
	"github.com/otfabric/go-canopen"
	"github.com/otfabric/go-canopen/mpdo"
	"github.com/otfabric/go-canopen/node"
	"github.com/otfabric/go-canopen/od"
)

// mpdoNodes starts built-in nodes 5 (with an object scanner list) and 6
// (with an object dispatcher list).
func mpdoNodes(t *testing.T) (producer, consumer *node.Node) {
	t.Helper()
	d5, err := builtinDictionary(5, false)
	if err != nil {
		t.Fatal(err)
	}
	sl, _ := mpdo.NewScannerList(1, 4)
	_ = d5.Add(sl)
	d6, _ := builtinDictionary(6, false)
	dl, _ := mpdo.NewDispatcherList(1, 4)
	_ = d6.Add(dl)
	return simNode(t, d5), simNode(t, d6)
}

func TestMPDO(t *testing.T) {
	fresh(t)
	p, c := mpdoNodes(t)
	v := []string{"-i", "virtual"}
	cmds := [][]string{
		{"mpdo", "configure", "5", "--tpdo", "2", "--mode", "sam", "--cob-id", "0x285"},
		{"mpdo", "configure", "6", "--rpdo", "2", "--mode", "SAM", "--cob-id", "285"},
		{"mpdo", "configure", "5", "--tpdo", "3", "--mode", "dam", "--cob-id", "0x385", "--inhibit", "10"},
		{"mpdo", "configure", "6", "--rpdo", "3", "--mode", "dam", "--cob-id", "0x385"},
	}
	for _, cmd := range cmds {
		want(t, run(t, "", append(cmd, v...)...), 0, "is an MPDO")
	}
	want(t, run(t, "", append([]string{"mpdo", "scanner", "5", "--list", "1", "--entry", "2001:00,2000:00", "--format", "json"}, v...)...), 0,
		`"command": "mpdo scanner"`, `"index": "0x1FA0"`, "2001:00+1")
	want(t, run(t, "", append([]string{"mpdo", "dispatcher", "6", "--entry", "5:2001:00=2001:00"}, v...)...), 0,
		"object dispatcher list 1 (0x1FD0): 1 entries written", "node 5 2001:00+1 -> 2001:00")
	if mpdo.ModeOf(p.Dictionary(), true, 2) != mpdo.ModeSAM || mpdo.ModeOf(c.Dictionary(), false, 3) != mpdo.ModeDAM {
		t.Fatal("modes")
	}
	want(t, run(t, "", "nmt", "start", "all", "-i", "virtual"), 0)
	eventually(t, "operational", func() bool { return c.State() == canopen.StateOperational && p.State() == canopen.StateOperational })
	wait := background("mpdo", "watch", "--cob-id", "0x285,0x385", "--count", "3", "--duration", "10s", "-i", "virtual", "--eds", "testdata/io401.eds")
	time.Sleep(50 * time.Millisecond)
	_ = p.Dictionary().SetUint(simOutput, 0, 0x5A)
	ctx := context.Background()
	eventually(t, "SAM sent", func() bool { return p.MPDOProducer(2).Send(ctx, simOutput, 0) == nil })
	eventually(t, "dispatched", func() bool { v, _ := c.Dictionary().GetUint(simOutput, 0); return v == 0x5A })
	if err := p.MPDOProducer(3).SendDAM(ctx, 6, simText, 0, []byte("abc")); err != nil {
		t.Fatal(err)
	}
	f, _ := canbus.NewStandard(0x285, []byte{1, 2})
	_ = port(t).Send(ctx, &f)
	r := wait()
	want(t, r, 0, "0x285 MPDO SAM from node 5 0x2001:00 Firmware image = 5a000000", "MPDO DAM to node 6 0x2002:00", "MPDO malformed", "3 MPDOs, 1 malformed")
	r = runWith(t, func() {
		_ = p.MPDOProducer(3).SendDAM(ctx, 0, simText, 0, []byte("x"))
	}, "mpdo", "watch", "--cob-id", "385", "--count", "1", "--duration", "5s", "-i", "virtual", "--format", "json")
	want(t, r, 0, `"type":"mpdo"`, `"mode":"dam"`, `"type":"summary"`)
	r = runWith(t, func() {
		_ = p.MPDOProducer(3).SendDAM(ctx, 0, simText, 0, []byte("x"))
	}, "mpdo", "watch", "--cob-id", "385", "--count", "1", "--duration", "5s", "-i", "virtual")
	want(t, r, 0, "to all nodes")
	want(t, run(t, "", "mpdo", "watch", "--cob-id", "0x18000385", "--duration", "10ms", "-i", "virtual"), 0, "0 MPDOs")
	// A device without lists refuses.
	want(t, run(t, "", append([]string{"mpdo", "scanner", "6", "--entry", "2000:00"}, v...)...), 11, "0x06020000")
	_ = od.IndexIdentity
}

// runWith runs a command in the background, calls produce once it is
// listening and returns its result.
func runWith(t *testing.T, produce func(), args ...string) result {
	t.Helper()
	wait := background(args...)
	time.Sleep(50 * time.Millisecond)
	produce()
	return wait()
}

func TestMPDOInvalid(t *testing.T) {
	fresh(t)
	for _, c := range []struct {
		args []string
		code int
		msg  string
	}{
		{[]string{"configure", "5", "--mode", "sam", "--cob-id", "285"}, 3, "--tpdo or --rpdo"},
		{[]string{"configure", "5", "--tpdo", "1", "--rpdo", "1", "--mode", "sam", "--cob-id", "285"}, 3, "--tpdo or --rpdo"},
		{[]string{"configure", "5", "--tpdo", "600", "--mode", "sam", "--cob-id", "285"}, 3, "1..512"},
		{[]string{"configure", "5", "--tpdo", "1", "--mode", "x", "--cob-id", "285"}, 3, "--mode"},
		{[]string{"configure", "5", "--tpdo", "1", "--mode", "sam", "--cob-id", "x"}, 3, "COB-ID"},
		{[]string{"configure", "5", "--tpdo", "1", "--mode", "sam", "--cob-id", "285", "--inhibit", "-1"}, 3, "--inhibit"},
		{[]string{"configure", "x", "--tpdo", "1", "--mode", "sam", "--cob-id", "285"}, 3, "node-ID"},
		{[]string{"configure", "5", "--tpdo", "1", "--mode", "sam", "--cob-id", "285", "--timeout", "0s"}, 3, "--timeout"},
		{[]string{"configure", "5", "--tpdo", "1", "--mode", "sam", "--cob-id", "285", "--brs"}, 3, "--brs needs --fd"},
		{[]string{"configure", "5", "--tpdo", "1", "--mode", "sam"}, 2, "cob-id"},
		{[]string{"configure", "5", "--tpdo", "1", "--mode", "sam", "--cob-id", "285", "-i", "slcan:/dev/null"}, 9, "--yes"},
		{[]string{"configure", "5", "--tpdo", "1", "--mode", "sam", "--cob-id", "285", "-i", "slcan:/nonexistent", "--yes"}, 4, ""},
		{[]string{"configure", "5", "--tpdo", "1", "--mode", "sam", "--cob-id", "285", "--timeout", "20ms"}, 5, ""},
		{[]string{"scanner", "x"}, 3, "node-ID"},
		{[]string{"scanner", "5", "--list", "0"}, 3, "--list"},
		{[]string{"scanner", "5", "--entry", "x"}, 3, "invalid object"},
		{[]string{"scanner", "5", "--entry", "0000:00"}, 3, "invalid scanner entry"},
		{[]string{"scanner", "5", "--entry", "2000:zz"}, 3, "sub-index"},
		{[]string{"scanner", "5", "--entry", "zz:00"}, 3, "invalid index"},
		{[]string{"scanner", "5", "--entry", "2000:00+0"}, 3, "block size"},
		{[]string{"scanner", "5", "--entry", "2000:00", "--client-id", "0x80"}, 3, "--client-id"},
		{[]string{"scanner", "5", "--entry", "2000:00", "--brs"}, 3, "--brs"},
		{[]string{"scanner", "5", "--entry", "2000:00", "-i", "slcan:/dev/null"}, 9, "--yes"},
		{[]string{"scanner", "5", "--entry", "2000:00", "-i", "slcan:/nonexistent", "--yes"}, 4, ""},
		{[]string{"dispatcher", "6", "--entry", "5:2000:00"}, 3, "NODE:IIII:SS"},
		{[]string{"dispatcher", "6", "--entry", "200:2000:00=2000:00"}, 3, "node-ID"},
		{[]string{"dispatcher", "6", "--entry", "5:x=2000:00"}, 3, "invalid object"},
		{[]string{"dispatcher", "6", "--entry", "5:2000:00=x"}, 3, "invalid object"},
		{[]string{"dispatcher", "6", "--entry", "5:2000:00=2000:00+2"}, 3, "block size goes"},
		{[]string{"dispatcher", "6", "--entry", "5:2000:FF+2=2000:00"}, 3, "invalid dispatcher entry"},
		{[]string{"dispatcher", "6", "--entry", "5:2000:00=0000:00"}, 3, "invalid dispatcher entry"},
		{[]string{"watch"}, 3, "--cob-id"},
		{[]string{"watch", "--cob-id", "x"}, 3, "COB-ID"},
		{[]string{"watch", "--cob-id", "285", "--count", "-1"}, 3, "--count"},
		{[]string{"watch", "--cob-id", "285", "--eds", "testdata/missing.eds"}, 3, "cannot read"},
		{[]string{"watch", "--cob-id", "285", "-i", "slcan:/nonexistent"}, 4, ""},
	} {
		args := append([]string{"mpdo"}, c.args...)
		if !contains(c.args, "-i") {
			args = append(args, "-i", "virtual")
		}
		want(t, run(t, "", args...), c.code, c.msg)
	}
	var entries []string
	for i := 0; i < 255; i++ {
		entries = append(entries, "--entry", "2000:00")
	}
	want(t, run(t, "", append([]string{"mpdo", "scanner", "5", "-i", "virtual"}, entries...)...), 3, "at most 254")
}
