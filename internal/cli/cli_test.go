// SPDX-License-Identifier: MIT

package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/otfabric/canopenctl/internal/errs"
	"github.com/otfabric/go-canopen"
	"github.com/otfabric/go-canopen/pdo"
)

func TestParseNode(t *testing.T) {
	for s, w := range map[string]canopen.NodeID{"1": 1, "127": 127, "0x7F": 127, " 5 ": 5} {
		if n, err := ParseNode(s, false); err != nil || n != w {
			t.Errorf("%q: %d %v", s, n, err)
		}
	}
	for _, s := range []string{"0", "128", "x", "", "all", "-1"} {
		if _, err := ParseNode(s, false); err == nil {
			t.Errorf("%q accepted", s)
		}
	}
	for _, s := range []string{"all", "ALL", "0"} {
		if n, err := ParseNode(s, true); err != nil || n != 0 {
			t.Errorf("%q: %d %v", s, n, err)
		}
	}
	if _, err := ParseNode("200", true); err == nil || !strings.Contains(err.Error(), "or all") {
		t.Fatal(err)
	}
}

func TestParseNodeList(t *testing.T) {
	l, err := ParseNodeList("5,1-3, 0x10,3")
	if err != nil || len(l) != 5 || l[0] != 1 || l[4] != 16 {
		t.Fatal(l, err)
	}
	for _, s := range []string{"", ",", "3-1", "1-200", "x", "0"} {
		if _, err := ParseNodeList(s); err == nil {
			t.Errorf("%q accepted", s)
		}
	}
}

func TestParseIndexSub(t *testing.T) {
	if i, err := ParseIndex("0x1018"); err != nil || i != 0x1018 {
		t.Fatal(i, err)
	}
	if i, err := ParseIndex("6041"); err != nil || i != 0x6041 {
		t.Fatal(i, err)
	}
	for _, s := range []string{"", "0x", "10000", "zz"} {
		if _, err := ParseIndex(s); err == nil {
			t.Errorf("%q accepted", s)
		}
	}
	if s, err := ParseSub("10"); err != nil || s != 10 {
		t.Fatal(s, err)
	}
	if s, err := ParseSub("0x10"); err != nil || s != 16 {
		t.Fatal(s, err)
	}
	if _, err := ParseSub("256"); err == nil {
		t.Fatal("256")
	}
}

func TestParseType(t *testing.T) {
	for s, w := range map[string]canopen.DataType{"u32": canopen.Unsigned32, "UNSIGNED16": canopen.Unsigned16, "0x0007": canopen.Unsigned32,
		"string": canopen.VisibleString, "f64": canopen.Real64, "bool": canopen.Boolean, "domain": canopen.Domain} {
		if dt, err := ParseType(s); err != nil || dt != w {
			t.Errorf("%q: %v %v", s, dt, err)
		}
	}
	for _, s := range []string{"u33", "0x0020", "0x1234", ""} {
		if _, err := ParseType(s); err == nil {
			t.Errorf("%q accepted", s)
		}
	}
}

func TestParseValue(t *testing.T) {
	cases := []struct {
		t    canopen.DataType
		s    string
		want []byte
	}{
		{canopen.Unsigned16, "1000", []byte{0xE8, 0x03}},
		{canopen.Unsigned32, "$NODEID+0x180", []byte{0x85, 0x01, 0, 0}},
		{canopen.Integer8, "-1", []byte{0xFF}},
		{canopen.Boolean, "true", []byte{1}},
		{canopen.Boolean, "off", []byte{0}},
		{canopen.VisibleString, "abc", []byte("abc")},
		{canopen.OctetString, "01 02", []byte{1, 2}},
		{canopen.Real32, "1.5", []byte{0, 0, 0xC0, 0x3F}},
	}
	for _, c := range cases {
		b, err := ParseValue(c.t, c.s, 5)
		if err != nil || !bytes.Equal(b, c.want) {
			t.Errorf("%s %q: %x %v", c.t, c.s, b, err)
		}
	}
	for _, c := range []struct {
		t canopen.DataType
		s string
	}{{canopen.Unsigned8, "256"}, {canopen.Unsigned8, ""}, {canopen.Integer8, "x"}, {canopen.OctetString, "0g"}, {canopen.Real32, "pi"}} {
		_, err := ParseValue(c.t, c.s, 5)
		var e *errs.Error
		if !errors.As(err, &e) || e.Code != errs.CodeInvalidValue {
			t.Errorf("%s %q: %v", c.t, c.s, err)
		}
	}
	if trimPrefix(errors.New("plain")) != "plain" {
		t.Fatal("trimPrefix")
	}
}

func TestParseMapping(t *testing.T) {
	m, err := ParseMapping("0x6041:00:16, 6064:0/32,")
	if err != nil || len(m) != 2 || m[1] != (pdo.Entry{Index: 0x6064, SubIndex: 0, Bits: 32}) {
		t.Fatal(m, err)
	}
	if m, err := ParseMapping(""); err != nil || len(m) != 0 {
		t.Fatal(m, err)
	}
	for _, s := range []string{"6041:00", "x:00:16", "6041:zz:16", "6041:00:0", "6041:00:65", "6041:00:32,6064:00:32,6065:00:8"} {
		if _, err := ParseMapping(s); err == nil {
			t.Errorf("%q accepted", s)
		}
	}
	many := strings.Repeat("0002:00:1,", 65)
	if _, err := ParseMapping(many); err == nil {
		t.Fatal("65 entries")
	}
}

func TestParseCOBID(t *testing.T) {
	if c, err := ParseCOBID("185"); err != nil || c.ID() != 0x185 || c.Extended() {
		t.Fatal(c, err)
	}
	if c, err := ParseCOBID("0x18FF0005"); err != nil || !c.Extended() {
		t.Fatal(c, err)
	}
	for _, s := range []string{"", "x", "0x20000000"} {
		if _, err := ParseCOBID(s); err == nil {
			t.Errorf("%q accepted", s)
		}
	}
}

func TestGuessAndFormat(t *testing.T) {
	for n, w := range map[int]canopen.DataType{1: canopen.Unsigned8, 2: canopen.Unsigned16, 3: canopen.Unsigned24, 4: canopen.Unsigned32,
		5: canopen.Unsigned40, 6: canopen.Unsigned48, 7: canopen.Unsigned56, 8: canopen.Unsigned64} {
		if g := GuessType(make([]byte, n)); g != w {
			t.Errorf("%d: %s", n, g)
		}
	}
	if GuessType([]byte("hello world")) != canopen.VisibleString || GuessType([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8}) != canopen.OctetString || GuessType(nil) != canopen.OctetString {
		t.Fatal("guess")
	}
	if v, ok := JSONValue(canopen.Real32, []byte{0, 0, 0xC0, 0x7F}); !ok || v != "NaN" {
		t.Fatal(v)
	}
	if v, ok := JSONValue(canopen.Real64, []byte{0, 0, 0, 0, 0, 0, 0xF0, 0x7F}); !ok || v != "+Inf" {
		t.Fatal(v)
	}
	if v, ok := JSONValue(canopen.Real64, []byte{0, 0, 0, 0, 0, 0, 0xF0, 0x3F}); !ok || v != 1.0 {
		t.Fatal(v)
	}
	if v, ok := JSONValue(canopen.OctetString, []byte{1, 2}); !ok || v != "0102" {
		t.Fatal(v)
	}
	if v, ok := JSONValue(canopen.Unsigned32, []byte{1, 2}); ok || v != "0102" {
		t.Fatal(v)
	}
	for _, c := range []struct {
		t    canopen.DataType
		b    []byte
		want string
	}{
		{canopen.Unsigned16, []byte{0xE8, 3}, "1000 (0x03E8)"}, {canopen.Integer16, []byte{0xFF, 0xFF}, "-1"},
		{canopen.Boolean, []byte{1}, "true"}, {canopen.Boolean, []byte{0}, "false"}, {canopen.Unsigned32, []byte{1}, "01 (not a UNSIGNED32)"},
	} {
		if s := ValueText(c.t, c.b); s != c.want {
			t.Errorf("%s %x: %q", c.t, c.b, s)
		}
	}
	if Key(0x1018, 1) != "0x1018:01" || Hex(0x85, 3) != "0x085" {
		t.Fatal("Key/Hex")
	}
}

func TestLoadEDSAndDictionary(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadEDS(filepath.Join(dir, "none.eds")); err == nil {
		t.Fatal("missing file")
	}
	bad := filepath.Join(dir, "bad.eds")
	_ = os.WriteFile(bad, []byte("orphan=1\n"), 0o644)
	var e *errs.Error
	if _, err := LoadEDS(bad); !errors.As(err, &e) || e.Code != errs.CodeInvalidEDS {
		t.Fatal(err)
	}
	good := filepath.Join("..", "..", "cmd", "testdata", "io401.eds")
	f, err := LoadEDS(good)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Dictionary(f, good, 5, false); err != nil {
		t.Fatal(err)
	}
	broken, err := LoadEDS(filepath.Join("..", "..", "cmd", "testdata", "broken.dcf"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Dictionary(broken, "broken.dcf", 5, false); !errors.As(err, &e) || e.Code != errs.CodeInvalidEDS {
		t.Fatal(err)
	}
}

func TestOpenTrace(t *testing.T) {
	r, closeFn, err := OpenTrace("-", strings.NewReader("(1.000000) vcan0 123#00\n"))
	if err != nil || r == nil {
		t.Fatal(err)
	}
	closeFn()
	if _, _, err := OpenTrace(filepath.Join(t.TempDir(), "x"), nil); err == nil {
		t.Fatal("missing")
	}
	p := filepath.Join(t.TempDir(), "bad")
	_ = os.WriteFile(p, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, 0o644)
	if _, _, err := OpenTrace(p, nil); err == nil {
		t.Fatal("bad")
	}
}

func TestTransmitCheck(t *testing.T) {
	if TransmitCheck(Virtual, false, "x") != nil || TransmitCheck("can0", true, "x") != nil {
		t.Fatal("virtual/yes")
	}
	var e *errs.Error
	if err := TransmitCheck("slcan:/dev/ttyACM0", false, "x"); !errors.As(err, &e) || e.Kind != errs.KindSafety {
		t.Fatal(err)
	}
	err := TransmitCheck("nonexistent0", false, "x")
	if !errors.As(err, &e) {
		t.Fatal(err)
	}
	if e.Kind != errs.KindUnsupported && e.Kind != errs.KindTransport && e.Kind != errs.KindSafety {
		t.Fatal(e)
	}
}

// FuzzParseValue: no value string panics, and an accepted one has the
// size of its data type.
func FuzzParseValue(f *testing.F) {
	for _, s := range []string{"1", "-1", "0x7F", "$NODEID+0x180", "true", "1.5e3", "0xFFFFFFFF", "01 02", "", "017"} {
		f.Add(uint16(canopen.Unsigned32), s)
		f.Add(uint16(canopen.Integer8), s)
	}
	f.Fuzz(func(t *testing.T, typ uint16, s string) {
		dt := canopen.DataType(typ)
		if !dt.Known() || dt >= canopen.PDOCommParam {
			return
		}
		b, err := ParseValue(dt, s, 5)
		if err != nil {
			return
		}
		if dt.Size() > 0 && len(b) != dt.Size() {
			t.Fatalf("%s %q: %d bytes", dt, s, len(b))
		}
		if dt == canopen.VisibleString && utf8.ValidString(s) && string(b) != s {
			t.Fatalf("string %q -> %q", s, b)
		}
	})
}

// FuzzParseMapping: no mapping string panics, and an accepted mapping
// fits a PDO.
func FuzzParseMapping(f *testing.F) {
	for _, s := range []string{"0x6041:00:16", "6041:00:16,6064:00:32", "0002:00:8/", "1:2:3", "::", "6041/00/16"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		m, err := ParseMapping(s)
		if err != nil {
			return
		}
		if m.Bits() > pdo.MaxBits || len(m) > pdo.MaxEntries {
			t.Fatalf("%q: %d bits, %d entries", s, m.Bits(), len(m))
		}
		for _, e := range m {
			if e.Bits == 0 {
				t.Fatalf("%q: zero-length entry", s)
			}
		}
	})
}

// FuzzParseNodeList: accepted lists hold valid node-IDs in ascending
// order without duplicates.
func FuzzParseNodeList(f *testing.F) {
	for _, s := range []string{"1-127", "5", "1,2,3", "0x10-0x20", "3-1"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		l, err := ParseNodeList(s)
		if err != nil {
			return
		}
		for i, n := range l {
			if !n.Valid() || (i > 0 && l[i-1] >= n) {
				t.Fatalf("%q: %v", s, l)
			}
		}
	})
}

// FuzzParseIndexSubType: the index, sub-index and type parsers never
// panic.
func FuzzParseIndexSubType(f *testing.F) {
	for _, s := range []string{"1018", "0x1018", "1", "0x01", "u32", "UNSIGNED32", "0x0007", "all"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = ParseIndex(s)
		_, _ = ParseSub(s)
		_, _ = ParseType(s)
		_, _ = ParseCOBID(s)
		_, _ = ParseNode(s, true)
	})
}

func TestParseCiA302Inputs(t *testing.T) {
	if v, err := ParseUint32("0x10"); err != nil || v != 16 {
		t.Fatal(v, err)
	}
	if _, err := ParseUint32("4294967296"); err == nil {
		t.Fatal("overflow")
	}
	a, err := ParseLSSAddress("F7A:0xC0C7:10000:00000005")
	if err != nil || a.Vendor != 0xF7A || a.Product != 0xC0C7 || a.Revision != 0x10000 || a.Serial != 5 {
		t.Fatal(a, err)
	}
	for _, bad := range []string{"", "1:2:3", "1:2:3:4:5", "x:2:3:4", "1:2:3:100000000"} {
		if _, err := ParseLSSAddress(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	se, err := ParseScannerEntry("2000:01+3")
	if err != nil || se.Index != 0x2000 || se.SubIndex != 1 || se.BlockSize != 3 {
		t.Fatal(se, err)
	}
	for _, bad := range []string{"2000", "2000:01+x", "2000:01+0", "0:0", "2000:FF+2", "x:00", "2000:x"} {
		if _, err := ParseScannerEntry(bad); err == nil {
			t.Errorf("scanner %q accepted", bad)
		}
	}
	de, err := ParseDispatcherEntry("5:2000:01+2=2100:03")
	if err != nil || de.Producer != 5 || de.ProducerIndex != 0x2000 || de.ProducerSubIndex != 1 || de.BlockSize != 2 || de.Index != 0x2100 || de.SubIndex != 3 {
		t.Fatal(de, err)
	}
	for _, bad := range []string{"5:2000:01", "2000:01=2100:00", "0:2000:01=2100:00", "5:x=2100:00", "5:2000:01=x", "5:2000:01=2100:00+2", "5:0:0=2100:00", "5:2000:FF+2=2100:00"} {
		if _, err := ParseDispatcherEntry(bad); err == nil {
			t.Errorf("dispatcher %q accepted", bad)
		}
	}
	for s, want := range map[string]uint8{"all": 1, "Communication": 2, "app": 3, "manufacturer": 4, "0x7F": 127, "5": 5} {
		if g, err := ParseStoreGroup(s); err != nil || g != want {
			t.Errorf("%q: %d %v", s, g, err)
		}
	}
	for _, bad := range []string{"0", "128", "x"} {
		if _, err := ParseStoreGroup(bad); err == nil {
			t.Errorf("group %q accepted", bad)
		}
	}
	if n, v, err := ParseNodeValue("5 = a.dcf"); err != nil || n != 5 || v != "a.dcf" {
		t.Fatal(n, v, err)
	}
	for _, bad := range []string{"5", "5=", "x=a"} {
		if _, _, err := ParseNodeValue(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// FuzzParseMPDOAndLSS checks that the LSS address and the MPDO list
// entries parse to valid values or are refused, and round-trip.
func FuzzParseMPDOAndLSS(f *testing.F) {
	for _, s := range []string{"F7A:C0C7:10000:5", "2000:01+3", "5:2000:01+2=2100:03", "1:2:3:4", "0:0=0:0", "::::", "+"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if a, err := ParseLSSAddress(s); err == nil {
			if b, err := ParseLSSAddress(a.String()); err != nil || b != a {
				t.Fatalf("%q: %v does not round-trip: %v", s, a, err)
			}
		}
		if e, err := ParseScannerEntry(s); err == nil && (e.Index == 0 || e.Validate() != nil) {
			t.Fatalf("%q: invalid scanner entry %+v accepted", s, e)
		}
		if e, err := ParseDispatcherEntry(s); err == nil && (e.Index == 0 || e.ProducerIndex == 0 || !e.Producer.Valid() || e.Validate() != nil) {
			t.Fatalf("%q: invalid dispatcher entry %+v accepted", s, e)
		}
	})
}
