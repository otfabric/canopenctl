// SPDX-License-Identifier: MIT

package output

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

type res struct{ N int }

func (r res) Text() string      { return "n=1\n" }
func (r res) Headers() []string { return []string{"A", "B"} }
func (r res) Rows() [][]string  { return [][]string{{"1", "22"}} }

type plain struct{}

func (plain) Text() string { return "plain" }

func TestFormats(t *testing.T) {
	for in, want := range map[string]Format{"": Text, "JSON": JSON, " table ": Table} {
		if f, err := Parse(in); err != nil || f != want {
			t.Fatal(in, f, err)
		}
	}
	if _, err := Parse("xml"); err == nil {
		t.Fatal("xml")
	}
	var b bytes.Buffer
	_ = Write(&b, Text, "c", res{})
	_ = Write(&b, Table, "c", res{})
	_ = Write(&b, Table, "c", plain{})
	_ = Write(&b, JSON, "c", res{N: 2})
	out := b.String()
	for _, w := range []string{"n=1\nA  B\n1  22\nplain\n", `"schemaVersion": "1.0"`, `"command": "c"`, `"N": 2`} {
		if !strings.Contains(out, w) {
			t.Fatalf("lacks %q:\n%s", w, out)
		}
	}
	b.Reset()
	_ = WriteError(&b, map[string]string{"kind": "x"})
	if !strings.Contains(b.String(), `"error"`) {
		t.Fatal(b.String())
	}
	if err := Write(&b, JSON, "c", bad{}); err == nil {
		t.Fatal("unencodable value")
	}
}

type bad struct{ C chan int }

func (bad) Text() string { return "" }

func TestStream(t *testing.T) {
	var b bytes.Buffer
	s := NewStream(&b, JSON)
	_ = s.Record("frame", "ignored", map[string]int{"id": 1})
	_ = s.Record("empty", "", struct{}{})
	if b.String() != `{"schemaVersion":"1.0","type":"frame","id":1}`+"\n"+`{"schemaVersion":"1.0","type":"empty"}`+"\n" {
		t.Fatal(b.String())
	}
	if err := s.Record("x", "", make(chan int)); err == nil {
		t.Fatal("unencodable")
	}
	b.Reset()
	_ = NewStream(&b, Text).Record("frame", "line", nil)
	if b.String() != "line\n" {
		t.Fatal(b.String())
	}
	_ = errors.New
}

func TestSanitize(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", ""},
		{"Producer heartbeat time", "Producer heartbeat time"},
		{"Grüße °C µs ✓", "Grüße °C µs ✓"},
		{`C:\dir`, `C:\dir`},
		{"a\x1b[2Jb", `a\x1b[2Jb`},
		{"bell\a", `bell\a`},
		{"line\nbreak\ttab\r", `line\nbreak\ttab\r`},
		{"\x00\x1f\x7f", `\x00\x1f\x7f`},
		{"c1 \u0085 \u009b", `c1 \u0085 \u009b`},
		{"name\u202Etxt.exe", `name\u202etxt.exe`},
		{"\u202A\u202B\u202C\u202D\u2066\u2067\u2068\u2069\u200E\u200F\u061C", `\u202a\u202b\u202c\u202d\u2066\u2067\u2068\u2069\u200e\u200f\u061c`},
		{"latin1 \xfc \x9b[2J", "latin1 \xfc " + `\x9b[2J`}, // a Latin-1 letter is kept, CSI is not
		{"\u2065\u2070\u2029", "\u2065\u2070\u2029"},        // around the controls: not escaped
	} {
		if got := Sanitize(tc.in); got != tc.want {
			t.Errorf("Sanitize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestSanitizedOutput: table cells and text stream lines are written
// through Sanitize; JSON escapes the same strings itself.
func TestSanitizedOutput(t *testing.T) {
	var b bytes.Buffer
	if err := WriteTable(&b, []string{"KEY", "NAME"}, [][]string{{"0x1008:00", "x\x1b]0;title\a\u202Ey"}}); err != nil {
		t.Fatal(err)
	}
	if got := b.String(); !strings.Contains(got, `x\x1b]0;title\a\u202ey`) || strings.ContainsAny(got, "\x1b\a\u202E") {
		t.Fatalf("table: %q", got)
	}
	for _, f := range []Format{Text, Table, JSON} {
		b.Reset()
		if err := NewStream(&b, f).Record("message", "(-) can0\x1b[2J 123#00", map[string]string{"interface": "can0\x1b[2J"}); err != nil {
			t.Fatal(err)
		}
		if got := b.String(); strings.ContainsRune(got, 0x1b) || !strings.Contains(got, `can0\x1b[2J`) && !strings.Contains(got, `can0\u001b[2J`) {
			t.Fatalf("%s stream: %q", f, got)
		}
	}
}
