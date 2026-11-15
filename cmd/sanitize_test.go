// SPDX-License-Identifier: MIT

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// controlled is a fixture with ESC, BEL and U+202E (right-to-left
// override) in the given lines.
func controlled(t *testing.T, name string, replace ...string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	for i := 0; i < len(replace); i += 2 {
		if !strings.Contains(s, replace[i]) {
			t.Fatalf("%s has no %q", name, replace[i])
		}
		s = strings.Replace(s, replace[i], replace[i+1], 1)
	}
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// terminalSafe fails when out has a C0 control other than newline and
// tab, DEL, a C1 control or a bidirectional control.
func terminalSafe(t *testing.T, what, out string) {
	t.Helper()
	for i, r := range out {
		if r < 0x20 && r != '\n' && r != '\t' || r >= 0x7F && r <= 0x9F ||
			r >= 0x202A && r <= 0x202E || r >= 0x2066 && r <= 0x2069 {
			t.Fatalf("%s: %U at byte %d:\n%q", what, r, i, out)
		}
	}
}

// TestControlCharactersEscaped: names and descriptions from EDS and DCF
// files, and interface names from a candump log, are written escaped in
// text and table output; JSON output keeps them, JSON-escaped.
func TestControlCharactersEscaped(t *testing.T) {
	fresh(t)
	ioNode(t, 5)
	const controls = "\x1b[2J\a\u202E"
	const escaped = `\x1b[2J\a\u202e`
	eds := controlled(t, "io401.eds",
		"ProductName=IO-401", "ProductName=IO-401"+controls,
		"VendorName=OT Fabric", "VendorName=OT Fabric"+controls,
		"Description=CiA 401 I/O module", "Description=CiA 401"+controls,
		"ParameterName=Manufacturer device name", "ParameterName=Manufacturer device name"+controls)
	dcf := controlled(t, "node37.dcf", "ParameterName=Producer heartbeat time", "ParameterName=Producer heartbeat time"+controls)
	trace := "(1700000000.000000) can0" + controls + " 705#7F\n"
	for _, tc := range []struct {
		stdin string
		args  []string
	}{
		{"", []string{"eds", "show", eds}},
		{"", []string{"eds", "show", eds, "--format", "table"}},
		{"", []string{"sdo", "dump", "5", "--eds", eds, "-i", "virtual"}},
		{"", []string{"sdo", "dump", "5", "--eds", eds, "-i", "virtual", "--format", "table"}},
		{"", []string{"sdo", "read", "5", "0x1008", "0", "--eds", eds, "-i", "virtual"}},
		{"", []string{"sdo", "read", "5", "0x1008", "0", "--eds", eds, "-i", "virtual", "--format", "table"}},
		{"", []string{"dcf", "apply", "37", dcf, "--dry-run"}},
		{"", []string{"cdcf", "show", dcf}},
		{"", []string{"cdcf", "show", dcf, "--format", "table"}},
		{trace, []string{"decode", "-"}},
	} {
		name := strings.Join(tc.args, " ")
		r := run(t, tc.stdin, tc.args...)
		if r.code != 0 {
			t.Fatalf("%s: exit %d\nstderr: %s", name, r.code, r.errOut)
		}
		terminalSafe(t, name, r.out+r.errOut)
		if !strings.Contains(r.out, escaped) {
			t.Errorf("%s: the name is not shown escaped (%s):\n%s", name, escaped, r.out)
		}
	}
	// JSON keeps the strings as they are in the file.
	want(t, run(t, "", "eds", "show", eds, "--format", "json"), 0, `"productName": "IO-401\u001b[2J\u0007`+"\u202E"+`"`)
	want(t, run(t, trace, "decode", "-", "--format", "json"), 0, `"interface":"can0\u001b[2J\u0007`+"\u202E"+`"`)
	// A name without control characters is written as before.
	want(t, run(t, "", "eds", "show", "testdata/io401.eds"), 0, "IO-401 CiA 401 I/O module (vendor OT Fabric", "Manufacturer device name")
}
