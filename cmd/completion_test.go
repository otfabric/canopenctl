// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// complete runs the hidden __complete command and returns the completions
// (descriptions dropped) and the directive.
func complete(t *testing.T, args ...string) ([]string, cobra.ShellCompDirective) {
	t.Helper()
	r := run(t, "", append([]string{cobra.ShellCompRequestCmd}, args...)...)
	if r.code != 0 {
		t.Fatalf("__complete %v: exit %d: %s", args, r.code, r.errOut)
	}
	lines := strings.Split(strings.TrimSpace(r.out), "\n")
	var d int
	if _, err := fmt.Sscanf(lines[len(lines)-1], ":%d", &d); err != nil {
		t.Fatalf("__complete %v: no directive: %q", args, r.out)
	}
	var vals []string
	for _, l := range lines[:len(lines)-1] {
		vals = append(vals, strings.SplitN(l, "\t", 2)[0])
	}
	return vals, cobra.ShellCompDirective(d)
}

func TestCompletionValues(t *testing.T) {
	fresh(t)
	for _, c := range []struct {
		args []string
		want []string
		dir  cobra.ShellCompDirective
	}{
		{[]string{"completion", ""}, []string{"bash", "zsh", "fish", "powershell"}, cobra.ShellCompDirectiveNoFileComp},
		{[]string{"version", "--format", ""}, []string{"text", "json", "table"}, cobra.ShellCompDirectiveNoFileComp},
		{[]string{"--slcan-bitrate", ""}, []string{"10000", "125000", "1000000", "0"}, cobra.ShellCompDirectiveNoFileComp},
		{[]string{"monitor", "--service", ""}, serviceList(), cobra.ShellCompDirectiveNoFileComp},
		{[]string{"decode", "--service", ""}, []string{"nmt", "sdo", "usdo"}, cobra.ShellCompDirectiveNoFileComp},
		{[]string{"sdo", "read", "--type", ""}, []string{"UNSIGNED32", "VISIBLE_STRING", "u32", "octets"}, cobra.ShellCompDirectiveNoFileComp},
		{[]string{"sdo", "write", "-t", ""}, []string{"REAL32", "f32"}, cobra.ShellCompDirectiveNoFileComp},
		{[]string{"nmt", ""}, []string{"start", "stop", "preop", "reset-node", "reset-comm"}, cobra.ShellCompDirectiveNoFileComp},
		{[]string{"nmt", "start", ""}, []string{"all"}, cobra.ShellCompDirectiveNoFileComp},
		{[]string{"eds", "convert", "--to", ""}, []string{"dcf", "eds"}, cobra.ShellCompDirectiveNoFileComp},
		{[]string{"eds", "show", ""}, []string{"eds", "dcf"}, cobra.ShellCompDirectiveFilterFileExt},
		{[]string{"pdo", "show", "5", "--eds", ""}, []string{"eds", "dcf"}, cobra.ShellCompDirectiveFilterFileExt},
		{[]string{"eds", "generate", "x.eds", "-o", ""}, []string{"go"}, cobra.ShellCompDirectiveFilterFileExt},
		{[]string{"lss", "switch", ""}, []string{"waiting", "configuration"}, cobra.ShellCompDirectiveNoFileComp},
		{[]string{"lss", "configure", "--bit-rate", ""}, []string{"1000", "125", "auto"}, cobra.ShellCompDirectiveNoFileComp},
		{[]string{"mpdo", "configure", "5", "--mode", ""}, []string{"dam", "sam"}, cobra.ShellCompDirectiveNoFileComp},
		{[]string{"store", "5", "--group", ""}, []string{"all", "communication", "application", "manufacturer"}, cobra.ShellCompDirectiveNoFileComp},
		{[]string{"restore", "5", "--group", ""}, []string{"all"}, cobra.ShellCompDirectiveNoFileComp},
		{[]string{"program", "control", "5", ""}, []string{"stop", "start", "reset", "clear"}, cobra.ShellCompDirectiveNoFileComp},
		{[]string{"cdcf", "apply", "5", ""}, []string{"cdcf", "dcf"}, cobra.ShellCompDirectiveFilterFileExt},
		{[]string{"cia402", "mode", "3", "--interface", ""}, nil, cobra.ShellCompDirectiveNoFileComp},
		{[]string{"sdo", "read", ""}, nil, cobra.ShellCompDirectiveNoFileComp},
		{[]string{"heartbeat", "watch", "--node", ""}, nil, cobra.ShellCompDirectiveNoFileComp},
		{[]string{"decode", ""}, nil, cobra.ShellCompDirectiveDefault},
	} {
		got, dir := complete(t, c.args...)
		if dir != c.dir {
			t.Errorf("%v: directive %d, want %d", c.args, dir, c.dir)
		}
		for _, w := range c.want {
			if !contains(got, w) {
				t.Errorf("%v: %q missing in %q", c.args, w, got)
			}
		}
		if c.want == nil && len(got) != 0 {
			t.Errorf("%v: want no values, got %q", c.args, got)
		}
	}
}

// Every flag that takes a value and every positional argument completes
// something deliberate: values, files of a type, or nothing. The tables
// name only commands and flags that exist.
func TestCompletionEverywhere(t *testing.T) {
	root := newRoot(&app{})
	seen := map[string]bool{}
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		path := strings.TrimPrefix(strings.TrimPrefix(c.CommandPath(), root.Name()), " ")
		seen[path] = true
		c.LocalFlags().VisitAll(func(f *pflag.Flag) {
			seen[path+" --"+f.Name] = true
			if f.Value.Type() == "bool" {
				return
			}
			if _, ok := c.GetFlagCompletionFunc(f.Name); !ok {
				t.Errorf("%q --%s (%s) has no completion", path, f.Name, f.Usage)
			}
		})
		if !c.HasSubCommands() && c.ValidArgsFunction == nil && len(c.ValidArgs) == 0 {
			t.Errorf("%q has no argument completion", path)
		}
		for _, s := range c.Commands() {
			walk(s)
		}
	}
	walk(root)
	for path, flags := range cmdFlagComps {
		for name := range flags {
			if !seen[path+" --"+name] {
				t.Errorf("cmdFlagComps: %q has no flag --%s", path, name)
			}
		}
	}
	for path := range argComps {
		if !seen[path] {
			t.Errorf("argComps: no command %q", path)
		}
	}
	for name := range flagComps {
		found := false
		for k := range seen {
			if strings.HasSuffix(k, " --"+name) || k == "--"+name {
				found = true
			}
		}
		if !found {
			t.Errorf("flagComps: no flag --%s", name)
		}
	}
}

// registerCompletions keeps a completion a command registered itself and
// gives the other value flags one.
func TestRegisterCompletionsKeepsOwn(t *testing.T) {
	root := &cobra.Command{Use: "canopenctl"}
	sub := &cobra.Command{Use: "x", RunE: func(*cobra.Command, []string) error { return nil }}
	sub.Flags().String("own", "", "")
	sub.Flags().String("other", "", "")
	own := func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"mine"}, cobra.ShellCompDirectiveKeepOrder
	}
	if err := sub.RegisterFlagCompletionFunc("own", own); err != nil {
		t.Fatal(err)
	}
	root.AddCommand(sub)
	registerCompletions(root)
	fn, ok := sub.GetFlagCompletionFunc("own")
	if !ok {
		t.Fatal("--own: no completion")
	}
	if vals, d := fn(sub, nil, ""); len(vals) != 1 || vals[0] != "mine" || d != cobra.ShellCompDirectiveKeepOrder {
		t.Errorf("--own: %q %d, want the registered completion", vals, d)
	}
	if _, ok := sub.GetFlagCompletionFunc("other"); !ok {
		t.Error("--other: no completion")
	}
}
