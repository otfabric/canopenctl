// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/otfabric/canopenctl/internal/cli"
	"github.com/otfabric/canopenctl/internal/errs"
	"github.com/otfabric/go-canopen"
	"github.com/otfabric/go-canopen/cia402"
	"github.com/otfabric/go-canopen/emcy"
	"github.com/otfabric/go-canopen/sdo"
	"github.com/spf13/cobra"
)

// driveFlags are the flags of the cia402 commands.
type driveFlags struct {
	bf          busFlags
	sf          sdoFlags
	stepTimeout time.Duration
	poll        time.Duration
}

func (d *driveFlags) add(c *cobra.Command, transmits bool) {
	d.bf.add(c, transmits)
	d.sf.add(c, false)
	c.Flags().DurationVar(&d.stepTimeout, "step-timeout", 2*time.Second, "Bound of every step: a state transition, a mode change, a set-point handshake, a move")
	c.Flags().DurationVar(&d.poll, "poll", 10*time.Millisecond, "Pause between two statusword reads")
}

// drive is an open controller.
type drive struct {
	node   canopen.NodeID
	ctl    *cia402.Controller
	client *sdo.Client
	close  func()
}

// open validates the flags, runs the safety check of a state-changing
// command (what != "") and opens a controller for the node.
func (d *driveFlags) open(arg, what string, resetFault bool) (*drive, error) {
	node, err := cli.ParseNode(arg, false)
	if err != nil {
		return nil, err
	}
	if err := d.sf.validate(); err != nil {
		return nil, err
	}
	if d.stepTimeout <= 0 || d.poll <= 0 {
		return nil, errs.Invalid("--step-timeout and --poll: more than 0")
	}
	if what != "" {
		if err := d.bf.check(what); err != nil {
			return nil, err
		}
	}
	bus, err := openBus(d.bf.iface, openOpts{})
	if err != nil {
		return nil, err
	}
	cl, err := sdoClient(bus, node, &d.sf)
	if err != nil {
		_ = bus.Close()
		return nil, err
	}
	ctl := cia402.NewController(sdoAdapter{cl}, &cia402.ControllerOptions{Timeout: d.stepTimeout, PollInterval: d.poll, ResetFault: resetFault})
	return &drive{node: node, ctl: ctl, client: cl, close: func() { _ = cl.Close(); _ = bus.Close() }}, nil
}

// driveState is the result of every cia402 command: the drive's state
// after it.
type driveState struct {
	Node       canopen.NodeID `json:"node"`
	Action     string         `json:"action,omitempty"`
	State      string         `json:"state"`
	Statusword string         `json:"statusword"`
	Status     []string       `json:"status"`
	Mode       *int8          `json:"mode,omitempty"`
	ModeName   string         `json:"modeName,omitempty"`
	Position   *int32         `json:"position,omitempty"`
	Velocity   *int32         `json:"velocity,omitempty"`
	FaultCode  string         `json:"faultCode,omitempty"`
	FaultText  string         `json:"faultText,omitempty"`
}

func (r driveState) Text() string {
	var b strings.Builder
	if r.Action != "" {
		fmt.Fprintf(&b, "%s done; ", r.Action)
	}
	fmt.Fprintf(&b, "node %d: %s (statusword %s: %s)", r.Node, r.State, r.Statusword, strings.Join(r.Status, ", "))
	if r.ModeName != "" {
		fmt.Fprintf(&b, "\nmode %s", r.ModeName)
	}
	if r.Position != nil {
		fmt.Fprintf(&b, "\nposition %d", *r.Position)
	}
	if r.Velocity != nil {
		fmt.Fprintf(&b, "\nvelocity %d", *r.Velocity)
	}
	if r.FaultCode != "" {
		fmt.Fprintf(&b, "\nfault %s %s", r.FaultCode, r.FaultText)
	}
	return b.String()
}

// statusBits names the set bits of a statusword (CiA 402 bit names; bit 5
// "quick stop" is active low: clear while a quick stop is in progress).
func statusBits(sw uint16) []string {
	names := []struct {
		bit  uint16
		name string
	}{
		{cia402.StatusReadyToSwitchOn, "ready to switch on"}, {cia402.StatusSwitchedOn, "switched on"},
		{cia402.StatusOperationEnabled, "operation enabled"}, {cia402.StatusFault, "fault"},
		{cia402.StatusVoltageEnabled, "voltage enabled"}, {cia402.StatusQuickStop, "quick stop"}, {cia402.StatusSwitchOnDisabled, "switch on disabled"},
		{cia402.StatusWarning, "warning"}, {cia402.StatusRemote, "remote"}, {cia402.StatusTargetReached, "target reached"},
		{cia402.StatusInternalLimitActive, "internal limit active"}, {cia402.StatusOMS12, "bit 12"}, {cia402.StatusOMS13, "bit 13"},
	}
	out := []string{}
	for _, n := range names {
		if sw&n.bit != 0 {
			out = append(out, n.name)
		}
	}
	return out
}

// snapshot reads the drive's state, mode, position, velocity and fault
// code; only the statusword is mandatory.
func (d *drive) snapshot(ctx context.Context, action string) (driveState, error) {
	sw, err := d.ctl.Statusword(ctx)
	if err != nil {
		return driveState{}, err
	}
	st := cia402.DecodeStatusword(sw)
	r := driveState{Node: d.node, Action: action, State: st.String(), Statusword: cli.Hex(uint64(sw), 4), Status: statusBits(sw)}
	if m, err := d.ctl.Mode(ctx); err == nil {
		v := int8(m)
		r.Mode, r.ModeName = &v, m.String()+" ("+m.Name()+")"
	}
	if p, err := d.ctl.Position(ctx); err == nil {
		r.Position = &p
	}
	if b, err := d.client.Upload(ctx, cia402.IndexVelocityActualValue, 0); err == nil && len(b) == 4 {
		v := int32(canopen.DecodeInt(b))
		r.Velocity = &v
	}
	if st == cia402.Fault || st == cia402.FaultReactionActive {
		if c, err := d.ctl.FaultCode(ctx); err == nil {
			r.FaultCode = cli.Hex(uint64(c), 4)
			r.FaultText = faultText(c)
		}
	}
	return r, nil
}

func faultText(c uint16) string {
	return emcy.CodeText(c)
}

// parseMode parses a mode of operation: an abbreviation (pp, pv, csp...)
// or a number (-128..127).
func parseMode(s string) (cia402.Mode, error) {
	for _, m := range []cia402.Mode{cia402.ModeNone, cia402.ModePP, cia402.ModeVL, cia402.ModePV, cia402.ModePT, cia402.ModeHM,
		cia402.ModeIP, cia402.ModeCSP, cia402.ModeCSV, cia402.ModeCST} {
		if strings.EqualFold(s, m.String()) {
			return m, nil
		}
	}
	v, err := strconv.ParseInt(s, 0, 8)
	if err != nil {
		return 0, errs.Invalid("invalid mode %q (pp, vl, pv, pt, hm, ip, csp, csv, cst, or a number)", s)
	}
	return cia402.Mode(v), nil
}

func newCiA402Cmd(a *app) *cobra.Command {
	c := &cobra.Command{
		Use:   "cia402",
		Short: "Operate a CiA 402 drive over SDO: state, enable, disable, mode, move, velocity",
		Long: `The cia402 commands operate a CiA 402 drive (IEC 61800-7-201) through its SDO
server: the controlword (0x6040) is written and the statusword (0x6041) read
back to verify every step of the state machine. Everything but state moves or
powers the drive: those commands need --yes on a real interface. A drive in
Fault, or one that does not reach a state within --step-timeout, ends the
command with exit code 12 (or 5) and the drive's state in the error.`,
	}
	type action struct {
		use, short, long, name string
		args                   int
		run                    func(ctx context.Context, d *drive, args []string) error
		extra                  func(c *cobra.Command)
	}
	var (
		resetFault                bool
		relative, immediate, wait bool
		vel, acc, dec             uint32
		moveTimeout               time.Duration
	)
	actions := []action{
		{use: "enable <node>", short: "Bring the drive to Operation enabled", name: "enable", args: 1,
			long: "enable walks the state machine to Operation enabled, verifying each state. A drive in Fault fails (exit 12) unless --reset-fault.",
			run:  func(ctx context.Context, d *drive, _ []string) error { return d.ctl.Enable(ctx) },
			extra: func(c *cobra.Command) {
				c.Flags().BoolVar(&resetFault, "reset-fault", false, "Reset a fault on the way")
			}},
		{use: "disable <node>", short: "Bring the drive to Switch on disabled (power stage off)", name: "disable", args: 1,
			run: func(ctx context.Context, d *drive, _ []string) error { return d.ctl.Disable(ctx) }},
		{use: "fault-reset <node>", short: "Reset a fault (rising edge of controlword bit 7)", name: "fault-reset", args: 1,
			run: func(ctx context.Context, d *drive, _ []string) error { return d.ctl.FaultReset(ctx) }},
		{use: "quick-stop <node>", short: "Send the quick stop command", name: "quick-stop", args: 1,
			run: func(ctx context.Context, d *drive, _ []string) error { return d.ctl.QuickStop(ctx) }},
		{use: "mode <node> <mode>", short: "Set the mode of operation (pp, pv, pt, hm, csp, csv, cst...)", name: "mode", args: 2,
			run: func(ctx context.Context, d *drive, args []string) error {
				m, err := parseMode(args[1])
				if err != nil {
					return err
				}
				return d.ctl.SetMode(ctx, m)
			}},
		{use: "move <node> <target>", short: "Profile position move to a target (increments)", name: "move", args: 2,
			long: "move switches to profile position mode when needed and moves to the target position with the set-point handshake; --wait waits for target reached. The drive must be in Operation enabled. Put -- before a negative target: cia402 move -i can0 --yes -- 3 -500.",
			run: func(ctx context.Context, d *drive, args []string) error {
				t, err := strconv.ParseInt(args[1], 0, 32)
				if err != nil {
					return errs.Invalid("invalid target %q (a 32-bit integer)", args[1])
				}
				return d.ctl.MoveTo(ctx, int32(t), &cia402.MoveOptions{Relative: relative, Immediate: immediate, Velocity: vel,
					Acceleration: acc, Deceleration: dec, Wait: wait, Timeout: moveTimeout})
			},
			extra: func(c *cobra.Command) {
				c.Flags().BoolVar(&relative, "relative", false, "Target relative to the preceding target")
				c.Flags().BoolVar(&immediate, "immediate", false, "Replace an active set-point (change set immediately)")
				c.Flags().BoolVar(&wait, "wait", false, "Wait for target reached")
				c.Flags().Uint32Var(&vel, "velocity", 0, "Profile velocity 0x6081 (0: keep)")
				c.Flags().Uint32Var(&acc, "accel", 0, "Profile acceleration 0x6083 (0: keep)")
				c.Flags().Uint32Var(&dec, "decel", 0, "Profile deceleration 0x6084 (0: keep)")
				c.Flags().DurationVar(&moveTimeout, "move-timeout", 0, "Bound of the wait for target reached (0: --step-timeout)")
			}},
		{use: "velocity <node> <velocity>", short: "Profile velocity: set the target velocity (increments/s)", name: "velocity", args: 2,
			long: "velocity switches to profile velocity mode when needed and writes the target velocity (0x60FF); it does not wait for the drive to reach it. The drive must be in Operation enabled. Put -- before a negative velocity.",
			run: func(ctx context.Context, d *drive, args []string) error {
				v, err := strconv.ParseInt(args[1], 0, 32)
				if err != nil {
					return errs.Invalid("invalid velocity %q (a 32-bit integer)", args[1])
				}
				return d.ctl.SetVelocity(ctx, int32(v))
			}},
	}
	for _, ac := range actions {
		ac := ac
		var df driveFlags
		sc := &cobra.Command{
			Use:   ac.use,
			Short: ac.short + " (needs --yes off virtual interfaces)",
			Long:  ac.long,
			Args:  cobra.ExactArgs(ac.args),
			RunE: func(cmd *cobra.Command, args []string) error {
				if ac.name == "mode" {
					if _, err := parseMode(args[1]); err != nil {
						return err
					}
				}
				d, err := df.open(args[0], "cia402 "+ac.name, resetFault)
				if err != nil {
					return err
				}
				defer d.close()
				if err := ac.run(cmd.Context(), d, args); err != nil {
					return err
				}
				r, err := d.snapshot(cmd.Context(), ac.name)
				if err != nil {
					return err
				}
				return a.emit("cia402 "+ac.name, r)
			},
		}
		df.add(sc, true)
		if ac.extra != nil {
			ac.extra(sc)
		}
		c.AddCommand(sc)
	}
	var sf driveFlags
	st := &cobra.Command{
		Use:   "state <node>",
		Short: "Read the drive's state, mode, position, velocity and fault code",
		Long:  "state reads the statusword and decodes the state, with the mode of operation, the actual position and velocity and, in Fault, the error code (0x603F). It only sends SDO read requests.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := sf.open(args[0], "", false)
			if err != nil {
				return err
			}
			defer d.close()
			r, err := d.snapshot(cmd.Context(), "")
			if err != nil {
				return err
			}
			return a.emit("cia402 state", r)
		},
	}
	sf.add(st, false)
	c.AddCommand(st)
	return c
}
