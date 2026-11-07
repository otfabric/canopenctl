// SPDX-License-Identifier: MIT

package cmd

import (
	"strings"

	"github.com/otfabric/canopenctl/internal/cli"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// compSpec is how one flag value or positional argument completes: a fixed
// set of values, files with one of a set of extensions, any file, or
// nothing (a free value: a node-ID, an index, a duration...).
type compSpec struct {
	values []string // fixed values ("value\tdescription" allowed)
	exts   []string // file extensions without the dot
	file   bool     // any file
}

func enum(values ...string) compSpec { return compSpec{values: values} }
func files(exts ...string) compSpec  { return compSpec{exts: exts, file: true} }

var (
	free    = compSpec{}
	anyFile = compSpec{file: true}
	edsFile = files("eds", "dcf")
)

// complete returns the completions of spec.
func (s compSpec) complete() ([]string, cobra.ShellCompDirective) {
	switch {
	case len(s.values) > 0:
		return s.values, cobra.ShellCompDirectiveNoFileComp
	case len(s.exts) > 0:
		return s.exts, cobra.ShellCompDirectiveFilterFileExt
	case s.file:
		return nil, cobra.ShellCompDirectiveDefault
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// flagComps are the completions of flags by name, on every command that
// has the flag.
var flagComps = map[string]compSpec{
	"format": enum("text\thuman-readable", "json\tversioned JSON", "table\taligned columns"),
	"eds":    edsFile,
}

// cmdFlagComps are the completions of flags of one command (its path
// without "canopenctl "); they win over flagComps.
var cmdFlagComps = map[string]map[string]compSpec{
	"monitor":      {"service": enum(serviceList()...)},
	"decode":       {"service": enum(serviceList()...)},
	"sdo read":     {"type": enum(cli.TypeNames()...), "output": anyFile},
	"sdo write":    {"type": enum(cli.TypeNames()...), "file": anyFile},
	"eds convert":  {"to": enum("dcf", "eds"), "output": edsFile},
	"eds generate": {"output": files("go")},
	"dcf export":   {"output": files("dcf")},
	"cdcf convert": {"output": files("cdcf")},
	"store":        {"group": enum("all", "communication", "application", "manufacturer")},
	"restore":      {"group": enum("all", "communication", "application", "manufacturer")},
	"lss configure": {"bit-rate": enum("1000", "800", "500", "250", "125", "50", "20", "10", "auto"),
		"node-id": enum("none")},
	"mpdo configure": {"mode": enum("dam\tdestination address mode", "sam\tsource address mode")},
	"simulate":       {"store": anyFile},
	"":               {"slcan-bitrate": enum("10000", "20000", "50000", "83300", "100000", "125000", "250000", "500000", "750000", "1000000", "0")},
}

// argComps are the completions of the positional arguments of a command,
// by position. A command not listed, and an argument past the specs, take
// free values (no file completion).
var argComps = map[string][]compSpec{
	"nmt":              {enum("start", "stop", "preop", "reset-node", "reset-comm"), enum("all")},
	"lss switch":       {enum("waiting", "configuration")},
	"program control":  {free, enum("stop", "start", "reset", "clear")},
	"program download": {free, anyFile},
	"decode":           {anyFile},
	"eds validate":     {edsFile},
	"eds show":         {edsFile},
	"eds convert":      {edsFile},
	"eds generate":     {edsFile},
	"dcf apply":        {free, files("dcf")},
	"cdcf convert":     {edsFile},
	"cdcf show":        {files("cdcf", "dcf")},
	"cdcf apply":       {free, files("cdcf", "dcf")},
}

// serviceList returns the --service names.
func serviceList() []string {
	return strings.Split(serviceNames(), ", ")
}

// registerCompletions gives every flag that takes a value, and the
// positional arguments of every command, a completion: fixed values, files
// of a type, or none for free values.
func registerCompletions(root *cobra.Command) {
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		path := strings.TrimPrefix(strings.TrimPrefix(c.CommandPath(), root.Name()), " ")
		c.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if f.Value.Type() == "bool" {
				return
			}
			if _, ok := c.GetFlagCompletionFunc(f.Name); ok {
				return
			}
			spec, ok := cmdFlagComps[path][f.Name]
			if !ok {
				spec = flagComps[f.Name]
			}
			_ = c.RegisterFlagCompletionFunc(f.Name, func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
				return spec.complete()
			})
		})
		if c.ValidArgsFunction == nil && len(c.ValidArgs) == 0 && !c.HasSubCommands() {
			specs := argComps[path]
			c.ValidArgsFunction = func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
				if len(args) >= len(specs) {
					return nil, cobra.ShellCompDirectiveNoFileComp
				}
				return specs[len(args)].complete()
			}
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
}
