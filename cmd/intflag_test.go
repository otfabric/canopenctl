// SPDX-License-Identifier: MIT

package cmd

import (
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/otfabric/canopenctl/internal/errs"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// TestIntFlagSet: the range of a 32-bit int on every platform. The table
// runs on linux/386 too (CI job test-386), where int has 32 bits.
func TestIntFlagSet(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    int
		invalid bool // refused as invalid input (exit 3)
		syntax  bool // refused by strconv, as pflag's int flag (exit 2)
	}{
		{in: "0", want: 0},
		{in: "127", want: 127},
		{in: "-1", want: -1},
		{in: "0x7f", want: 127},
		{in: "2147483647", want: 2147483647},
		{in: "0x7fffffff", want: 2147483647},
		{in: "-2147483648", want: -2147483648},
		{in: "2147483648", invalid: true},
		{in: "-2147483649", invalid: true},
		{in: "4294967296", invalid: true},
		{in: "4294967301", invalid: true}, // 5 in the low 32 bits
		{in: "0x100000005", invalid: true},
		{in: "9223372036854775807", invalid: true},
		{in: "-9223372036854775808", invalid: true},
		{in: "9223372036854775808", syntax: true},
		{in: "", syntax: true},
		{in: "five", syntax: true},
		{in: "1.5", syntax: true},
	} {
		v := 42
		f := &intFlag{p: &v, name: "node-id"}
		err := f.Set(tc.in)
		var e *errs.Error
		switch {
		case tc.invalid:
			if !errors.As(err, &e) || e.Kind != errs.KindInvalidInput || !strings.Contains(e.Message, "--node-id "+tc.in+": out of range (-2147483648..2147483647)") || v != 42 {
				t.Errorf("Set(%q) = %v, value %d; want invalid input, value unchanged", tc.in, err, v)
			}
		case tc.syntax:
			if err == nil || errors.As(err, &e) {
				t.Errorf("Set(%q) = %v; want strconv's error", tc.in, err)
			}
		default:
			if err != nil || v != tc.want || f.String() != strconv.Itoa(tc.want) {
				t.Errorf("Set(%q) = %v, value %d (%s); want %d", tc.in, err, v, f.String(), tc.want)
			}
		}
	}
	if (&intFlag{p: new(int)}).Type() != "int" {
		t.Fatal("type is not int: --help and completion show it")
	}
}

// intFlags returns every integer flag of the command tree, with the
// arguments that reach it.
func intFlags(t *testing.T) map[string][]string {
	t.Helper()
	root := newRoot(&app{in: strings.NewReader(""), out: io.Discard, errOut: io.Discard})
	found := map[string][]string{}
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		path := strings.Fields(c.CommandPath())[1:]
		visit := func(f *pflag.Flag) {
			if f.Value.Type() != "int" {
				return
			}
			if _, ok := f.Value.(*intFlag); !ok {
				t.Errorf("%s --%s is pflag's int flag, not an intFlag", c.CommandPath(), f.Name)
			}
			found[strings.Join(append(path, "--"+f.Name), " ")] = append(append([]string{}, path...), "--"+f.Name)
		}
		c.LocalNonPersistentFlags().VisitAll(visit)
		c.PersistentFlags().VisitAll(visit)
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
	return found
}

// TestIntFlagsRange: every integer flag of every command refuses a value
// outside the 32-bit range with exit 3, before the command runs; on
// linux/386 4294967301 was 5 and reached the command as a valid node-ID.
func TestIntFlagsRange(t *testing.T) {
	flags := intFlags(t)
	if len(flags) < 30 {
		t.Fatalf("%d integer flags found, want them all", len(flags))
	}
	for name, args := range flags {
		for _, v := range []string{"2147483648", "4294967301", "-2147483649"} {
			fresh(t)
			r := run(t, "", append(args, v, "--format", "json")...)
			want(t, r, 3, `"kind": "invalid_input"`, "out of range (-2147483648..2147483647)")
			if strings.Contains(r.out, `"type":`) || strings.Contains(r.out, `"command":`) {
				t.Fatalf("%s %s: the command ran: %s", name, v, r.out)
			}
		}
	}
}

// TestIntFlagsBoundary: the examples of the review, which on linux/386
// were the node-ID, PDO, program, counter or size of their low 32 bits. Each
// is refused with exit 3 before the bus is opened; the largest 32-bit value
// reaches the command's own range check (exit 3 with its message), and a
// valid value works as before.
func TestIntFlagsBoundary(t *testing.T) {
	for _, tc := range []struct {
		args    []string // the command, its flag last
		over    string   // wraps to a valid value with a 32-bit int
		ownMsg  string   // the command's own message for 2147483647
		valid   string
		validOK string // output of the valid value
	}{
		{[]string{"simulate", "-i", "virtual", "--duration", "10ms", "--node-id"}, "4294967301", "1..127", "5", "node 5 running"},
		{[]string{"pdo", "map", "5", "--map", "6000:01:8", "-i", "virtual", "--tpdo"}, "4294967298", "1..512", "", ""},
		{[]string{"program", "control", "5", "start", "-i", "virtual", "--program"}, "4294967297", "1..254", "", ""},
		{[]string{"lss", "assign-all", "-i", "virtual", "--first"}, "4294967306", "", "", ""},
		{[]string{"mpdo", "configure", "5", "--mode", "producer", "--cob-id", "0x285", "-i", "virtual", "--tpdo"}, "4294967297", "1..512", "", ""},
		{[]string{"sync", "-i", "virtual", "--duration", "10ms", "--counter"}, "4294967298", "2..240", "", ""},
		{[]string{"sdo", "read", "5", "0x1008", "0", "-i", "virtual", "--max-size"}, "4294967306", "1..1073741824", "64", "OTF IO-401"},
	} {
		name := strings.Join(tc.args, " ")
		fresh(t)
		ioNode(t, 5)
		r := run(t, "", append(tc.args, tc.over)...)
		want(t, r, 3, "out of range (-2147483648..2147483647)")
		r = run(t, "", append(tc.args, "2147483647")...)
		if r.code != 3 || tc.ownMsg != "" && !strings.Contains(r.errOut, tc.ownMsg) {
			t.Errorf("%s 2147483647: exit %d, want 3 from the command's own range check (%q)\nstderr: %s", name, r.code, tc.ownMsg, r.errOut)
		}
		if strings.Contains(r.errOut, "(-2147483648..2147483647)") {
			t.Errorf("%s 2147483647 refused by the flag, want the command's check: %s", name, r.errOut)
		}
		if tc.valid != "" {
			want(t, run(t, "", append(tc.args, tc.valid)...), 0, tc.validOK)
		}
	}
}
