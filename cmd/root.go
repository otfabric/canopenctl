// SPDX-License-Identifier: MIT

// Package cmd is the command tree of canopenctl.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/otfabric/canopenctl/internal/errs"
	"github.com/otfabric/canopenctl/internal/output"
	"github.com/otfabric/go-canbus/slcan"
	"github.com/spf13/cobra"
)

var buildMeta struct{ version, tag, commit, buildDate string }

// SetBuildMeta records link-time metadata (main's -X flags).
func SetBuildMeta(version, tag, commit, buildDate string) {
	buildMeta.version, buildMeta.tag, buildMeta.commit, buildMeta.buildDate = version, tag, commit, buildDate
}

// app is the state of one invocation.
type app struct {
	in      io.Reader
	out     io.Writer
	errOut  io.Writer
	format  output.Format
	started bool // RunE was reached: flags parsed
}

// formatOf reads --format before anything else, so that usage errors can
// follow it too.
func formatOf(args []string) output.Format {
	v := os.Getenv("CANOPENCTL_FORMAT")
	for i, a := range args {
		switch {
		case a == "--format" && i+1 < len(args):
			v = args[i+1]
		case strings.HasPrefix(a, "--format="):
			v = strings.TrimPrefix(a, "--format=")
		}
	}
	f, err := output.Parse(v)
	if err != nil {
		return output.Text
	}
	return f
}

const rootLong = `canopenctl is a command-line toolkit for CANopen (CiA 301, CiA 302 network
management, CiA 305 LSS, CiA 306 EDS/DCF, CiA 402 drives, CiA 1301 CANopen
FD), built on github.com/otfabric/go-canopen and go-canbus.

Read-only by default: monitor, decode, heartbeat watch, emcy watch, pdo
watch and mpdo watch never transmit; sdo read, sdo dump, scan, pdo show, dcf
export, program status and cia402 state only send SDO read requests.
Commands that change a device or the network (sdo write, nmt, pdo map, dcf
apply, cdcf apply, cia402 enable/disable/fault-reset/quick-stop/mode/move/
velocity, sync, time, lss, mpdo configure/scanner/dispatcher, boot, store,
restore, program download/control, simulate) need --yes on any interface
that is not virtual (vcan, vxcan or the in-process bus "virtual"), and are
bounded (--count, --duration, --timeout). --fd selects CANopen FD (USDO
instead of SDO, CAN FD frames) where a command offers it.

Interfaces: a SocketCAN interface (can0, vcan0; Linux), a serial-line CAN
adapter as slcan:<device> (slcan:/dev/ttyACM0, slcan:COM3) or
slcan:tcp://host:port, opened at --slcan-bitrate (passive commands open it
listen-only), or "virtual", an in-process bus for tests and simulation.

Results go to stdout as text (default), a table or JSON (--format); progress
and diagnostics go to stderr. Exit codes: docs/exitcodes.md.`

func newRoot(a *app) *cobra.Command {
	root := &cobra.Command{
		Use:           "canopenctl",
		Short:         "Monitor, configure, test and simulate CANopen networks",
		Long:          rootLong,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			f, err := output.Parse(cmd.Flag("format").Value.String())
			if err != nil {
				return errs.Invalid("%v", err)
			}
			a.format = f
			a.started = true
			br, _ := cmd.Flags().GetInt("slcan-bitrate")
			baud, _ := cmd.Flags().GetInt("slcan-baud")
			if _, ok := slcan.Bitrates[br]; br != 0 && !ok {
				return errs.Invalid("--slcan-bitrate %d: one of 10000, 20000, 50000, 83300, 100000, 125000, 250000, 500000, 750000, 1000000 or 0 (keep the adapter's)", br)
			}
			setSlcan(slcanFlags{bitrate: br, baud: baud})
			return nil
		},
	}
	root.PersistentFlags().String("format", envOr("CANOPENCTL_FORMAT", "text"), "Output format: text, json or table (env CANOPENCTL_FORMAT)")
	_ = root.RegisterFlagCompletionFunc("format", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"text\thuman-readable", "json\tversioned JSON", "table\taligned columns"}, cobra.ShellCompDirectiveNoFileComp
	})
	root.PersistentFlags().Int("slcan-bitrate", 500000, "Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's)")
	root.PersistentFlags().Int("slcan-baud", 115200, "Serial baud rate of an slcan adapter (USB adapters ignore it)")
	root.AddCommand(
		newMonitorCmd(a), newDecodeCmd(a), newSDOCmd(a), newNMTCmd(a), newHeartbeatCmd(a), newScanCmd(a),
		newEMCYCmd(a), newSyncCmd(a), newTimeCmd(a), newPDOCmd(a), newEDSCmd(a), newDCFCmd(a),
		newCiA402Cmd(a), newLSSCmd(a), newMPDOCmd(a), newBootCmd(a), newCDCFCmd(a), newStoreCmd(a, false), newStoreCmd(a, true),
		newProgramCmd(a), newSimulateCmd(a), newVersionCmd(a), newCompletionCmd(),
	)
	return root
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// emit writes a result in the selected format.
func (a *app) emit(command string, r output.Result) error {
	if err := output.Write(a.out, a.format, command, r); err != nil {
		return errs.New(errs.KindOutput, errs.CodeOutputFailed, "write output: %w", err)
	}
	return nil
}

// stream returns a record stream on stdout.
func (a *app) stream() *output.Stream { return output.NewStream(a.out, a.format) }

// warn writes a diagnostic line to stderr.
func (a *app) warn(format string, args ...any) {
	_, _ = fmt.Fprintf(a.errOut, "canopenctl: "+format+"\n", args...)
}

// Run executes canopenctl with args and returns the exit code.
func Run(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) int {
	a := &app{in: in, out: out, errOut: errOut, format: formatOf(args)}
	root := newRoot(a)
	root.SetArgs(args)
	root.SetIn(in)
	root.SetOut(out)
	root.SetErr(errOut)
	err := root.ExecuteContext(ctx)
	if err == nil {
		return errs.ExitOK
	}
	if errors.Is(err, errs.ErrFindings) {
		return errs.ExitFindings
	}
	var e *errs.Error
	if !a.started || isUsage(err) {
		e = &errs.Error{Kind: errs.KindUsage, Code: errs.CodeInvalidInput, Message: err.Error(), Cause: err}
		var typed *errs.Error
		if errors.As(err, &typed) {
			e = typed
		}
	} else {
		e = errs.Classify(err)
	}
	if a.format == output.JSON {
		_ = output.WriteError(out, e)
	} else {
		_, _ = fmt.Fprintf(errOut, "error: %s\n", e.Message)
		if e.Abort != nil && e.Abort.Text != "" && !strings.Contains(e.Message, e.Abort.Text) {
			_, _ = fmt.Fprintf(errOut, "abort %s: %s\n", e.Abort.Code, e.Abort.Text)
		}
		if e.Kind == errs.KindUsage {
			_, _ = fmt.Fprintln(errOut, "Run 'canopenctl --help' for usage.")
		}
	}
	return errs.ExitCode(e.Kind)
}

// isUsage recognises cobra's own errors about the command line, which
// can also arrive after flags were parsed (required flags are checked
// late).
func isUsage(err error) bool {
	var typed *errs.Error
	if errors.As(err, &typed) {
		return false
	}
	msg := err.Error()
	for _, p := range []string{"required flag", "accepts ", "requires at least", "unknown command", "unknown flag",
		"unknown shorthand", "invalid argument", "flag needs an argument", "if any flags in the group", "none of the others can be"} {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}

// Execute runs canopenctl on the process's arguments, ending on SIGINT or
// SIGTERM.
func Execute() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return Run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
}

type versionResult struct {
	Version   string `json:"version"`
	Tag       string `json:"tag,omitempty"`
	Commit    string `json:"commit,omitempty"`
	BuildDate string `json:"buildDate,omitempty"`
	GoCanopen string `json:"goCanopen"`
	GoCanbus  string `json:"goCanbus"`
	Go        string `json:"go"`
	Platform  string `json:"platform"`
}

func (v versionResult) Text() string {
	s := "canopenctl " + v.Version
	if v.Commit != "" {
		s += " (" + v.Commit + ")"
	}
	return s + "\ngo-canopen " + v.GoCanopen + "\ngo-canbus " + v.GoCanbus + "\n" + v.Go + " " + v.Platform
}

// depVersion returns the version of a dependency in the build info.
func depVersion(info *debug.BuildInfo, path string) string {
	for _, d := range info.Deps {
		if d.Path == path {
			if d.Replace != nil {
				if d.Replace.Version != "" {
					return d.Replace.Version
				}
				return d.Version + " (replaced by " + d.Replace.Path + ")"
			}
			return d.Version
		}
	}
	return "(unknown)"
}

func newVersionCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version of canopenctl, go-canopen and go-canbus",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			v := versionResult{Version: buildMeta.version, Tag: buildMeta.tag, Commit: buildMeta.commit, BuildDate: buildMeta.buildDate,
				GoCanopen: "(unknown)", GoCanbus: "(unknown)", Go: runtime.Version(), Platform: runtime.GOOS + "/" + runtime.GOARCH}
			if v.Version == "" {
				v.Version = "dev"
			}
			if info, ok := debug.ReadBuildInfo(); ok {
				v.GoCanopen = depVersion(info, "github.com/otfabric/go-canopen")
				v.GoCanbus = depVersion(info, "github.com/otfabric/go-canbus")
			}
			return a.emit("version", v)
		},
	}
}

func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "completion bash|zsh|fish|powershell",
		Short:     "Generate a shell completion script",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			root, out := cmd.Root(), cmd.OutOrStdout()
			switch args[0] {
			case "bash":
				return root.GenBashCompletionV2(out, true)
			case "zsh":
				return root.GenZshCompletion(out)
			case "fish":
				return root.GenFishCompletion(out, true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(out)
			}
			return errs.Invalid("unknown shell %q (bash, zsh, fish or powershell)", args[0])
		},
	}
}
