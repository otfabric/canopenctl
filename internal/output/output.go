// SPDX-License-Identifier: MIT

// Package output renders results as text, tables or versioned JSON
// (docs/json-output.md).
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"
	"unicode/utf8"
)

// SchemaVersion of every JSON document canopenctl writes.
const SchemaVersion = "1.0"

// Format is an output format.
type Format string

// Formats.
const (
	Text  Format = "text"
	JSON  Format = "json"
	Table Format = "table"
)

// Parse validates a --format value ("" is text).
func Parse(s string) (Format, error) {
	switch f := Format(strings.ToLower(strings.TrimSpace(s))); f {
	case "":
		return Text, nil
	case Text, JSON, Table:
		return f, nil
	}
	return "", fmt.Errorf("invalid --format %q (text, json or table)", s)
}

// Result is a command result.
type Result interface {
	// Text renders the human-readable form.
	Text() string
}

// Tabular results can also render as a table.
type Tabular interface {
	Headers() []string
	Rows() [][]string
}

// Envelope is the JSON document of a result.
type Envelope struct {
	SchemaVersion string `json:"schemaVersion"`
	Command       string `json:"command"`
	Result        any    `json:"result"`
}

// ErrorEnvelope is the JSON document of a fatal error.
type ErrorEnvelope struct {
	SchemaVersion string `json:"schemaVersion"`
	Error         any    `json:"error"`
}

// Write renders r.
func Write(w io.Writer, f Format, command string, r Result) error {
	switch f {
	case JSON:
		return writeJSON(w, Envelope{SchemaVersion, command, r})
	case Table:
		if t, ok := r.(Tabular); ok {
			return WriteTable(w, t.Headers(), t.Rows())
		}
	default:
	}
	s := strings.TrimSuffix(r.Text(), "\n")
	_, err := io.WriteString(w, s+"\n")
	return err
}

// WriteError renders a fatal error document.
func WriteError(w io.Writer, e any) error {
	return writeJSON(w, ErrorEnvelope{SchemaVersion, e})
}

func writeJSON(w io.Writer, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}

// WriteTable writes aligned columns. Every cell is written through
// Sanitize.
func WriteTable(w io.Writer, headers []string, rows [][]string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, strings.Join(headers, "\t"))
	cells := []string{}
	for _, r := range rows {
		cells = cells[:0]
		for _, c := range r {
			cells = append(cells, Sanitize(c))
		}
		_, _ = fmt.Fprintln(tw, strings.Join(cells, "\t"))
	}
	return tw.Flush()
}

// Sanitize returns s for a terminal: the C0 controls (newline and tab
// included), DEL, the C1 controls (U+0080..U+009F, and the bytes
// 0x80..0x9F that are not part of UTF-8, as in a Latin-1 file) and the
// bidirectional controls (U+061C, U+200E, U+200F, U+202A..U+202E,
// U+2066..U+2069) are written as Go escapes (\x1b, \u202e). Other bytes
// that are not UTF-8 (Latin-1 letters of an EDS) are kept. Text and table
// output pass strings from EDS, DCF and trace files through it; JSON
// output escapes them itself. A string without them is returned as it is.
func Sanitize(s string) string {
	clean := true
	for i := 0; i < len(s); i++ {
		if c := s[i]; c < 0x20 || c >= 0x7F {
			clean = false
			break
		}
	}
	if clean {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && n == 1:
			if c := s[i]; c <= 0x9F {
				fmt.Fprintf(&b, `\x%02x`, c)
			} else {
				b.WriteByte(c)
			}
		case r < 0x20 || r == 0x7F:
			b.WriteString(strings.Trim(strconv.QuoteRune(r), "'"))
		case r >= 0x80 && r <= 0x9F, r == 0x061C, r == 0x200E, r == 0x200F,
			r >= 0x202A && r <= 0x202E, r >= 0x2066 && r <= 0x2069:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteString(s[i : i+n])
		}
		i += n
	}
	return b.String()
}

// Stream writes one record per line: a line of text, or with JSON one
// compact JSON object per line (JSON Lines) carrying schemaVersion and
// type.
type Stream struct {
	w io.Writer
	f Format
}

// NewStream returns a record stream.
func NewStream(w io.Writer, f Format) *Stream { return &Stream{w: w, f: f} }

// Record writes one record: text for text and table output, through
// Sanitize, typ and fields for JSON.
func (s *Stream) Record(typ string, text string, fields any) error {
	if s.f != JSON {
		_, err := io.WriteString(s.w, Sanitize(text)+"\n")
		return err
	}
	b, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	// Splice schemaVersion and type in front of the record's own fields.
	head := fmt.Sprintf(`{"schemaVersion":%q,"type":%q`, SchemaVersion, typ)
	body := strings.TrimPrefix(strings.TrimSpace(string(b)), "{")
	if body == "}" {
		_, err = io.WriteString(s.w, head+"}\n")
	} else {
		_, err = io.WriteString(s.w, head+","+body+"\n")
	}
	return err
}
