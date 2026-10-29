// SPDX-License-Identifier: MIT

// Package output renders results as text, tables or versioned JSON
// (docs/json-output.md).
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
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

// WriteTable writes aligned columns.
func WriteTable(w io.Writer, headers []string, rows [][]string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, strings.Join(headers, "\t"))
	for _, r := range rows {
		_, _ = fmt.Fprintln(tw, strings.Join(r, "\t"))
	}
	return tw.Flush()
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

// Record writes one record: text for text and table output, typ and
// fields for JSON.
func (s *Stream) Record(typ string, text string, fields any) error {
	if s.f != JSON {
		_, err := io.WriteString(s.w, text+"\n")
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
