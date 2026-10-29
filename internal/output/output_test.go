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
