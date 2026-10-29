// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/otfabric/canopenctl/internal/cli"
	"github.com/otfabric/canopenctl/internal/errs"
	"github.com/otfabric/go-canopen"
	"github.com/otfabric/go-canopen/cia402"
	"github.com/otfabric/go-canopen/emcy"
	"github.com/otfabric/go-canopen/nmt"
	"github.com/otfabric/go-canopen/node"
	"github.com/otfabric/go-canopen/od"
	"github.com/spf13/cobra"
)

// Objects of the built-in simulated device (without --eds).
const (
	simCounter uint16 = 0x2000 // UNSIGNED32 ro, mappable: what --tick 0x2000:00 increments
	simOutput  uint16 = 0x2001 // UNSIGNED8 rw, mappable
	simText    uint16 = 0x2002 // VISIBLE_STRING rw
	simDomain  uint16 = 0x2003 // DOMAIN rw: segmented and block transfers
)

// builtinDictionary is the device simulate runs without --eds: the CiA
// 301 communication objects (4 RPDOs and TPDOs, heartbeat 1000 ms,
// consumer heartbeat, error history, node guarding, TIME) and a few
// application objects, TPDO1 mapping the counter 0x2000 (event-driven).
func builtinDictionary(id canopen.NodeID, drive bool) (*od.Dictionary, error) {
	cfg := od.Config{NodeID: id, VendorID: 0x00000F7A, ProductCode: 0x0000C0C7, Revision: 0x00010000, SerialNumber: uint32(id),
		DeviceName: "canopenctl simulated node", HardwareVersion: "virtual", SoftwareVersion: "canopenctl",
		HeartbeatMs: 1000, HeartbeatConsumers: 4, ErrorHistory: 8, RPDOs: 4, TPDOs: 4, NodeGuarding: true, Time: true}
	if drive {
		cfg.DeviceType = 0x00020192 // CiA 402, servo drive
		cfg.DeviceName = "canopenctl simulated CiA 402 drive"
	}
	d, err := od.Standard(cfg)
	if err != nil {
		return nil, err
	}
	counter := od.NewVar(simCounter, "Counter", canopen.Unsigned32, od.AccessRO, nil)
	counter.Subs[0].PDOMappable = true
	out := od.NewVar(simOutput, "Output", canopen.Unsigned8, od.AccessRW, nil)
	out.Subs[0].PDOMappable = true
	for _, o := range []*od.Object{counter, out,
		od.NewVar(simText, "Text", canopen.VisibleString, od.AccessRW, "hello"),
		od.NewVar(simDomain, "Data", canopen.Domain, od.AccessRW, nil),
	} {
		if err := d.Add(o); err != nil {
			return nil, err
		}
	}
	// TPDO1: the counter, event-driven (254).
	for _, kv := range []struct {
		index uint16
		sub   uint8
		v     uint64
	}{{od.IndexTPDOMapping, 1, uint64(simCounter)<<16 | 32}, {od.IndexTPDOMapping, 0, 1}, {od.IndexTPDOComm, 2, 254},
		{od.IndexRPDOMapping, 1, uint64(simOutput)<<16 | 8}, {od.IndexRPDOMapping, 0, 1}} {
		if err := d.SetUint(kv.index, kv.sub, kv.v); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// tickTarget is an object simulate increments.
type tickTarget struct {
	index uint16
	sub   uint8
	mask  uint64
}

func parseTicks(specs []string, d *od.Dictionary) ([]tickTarget, error) {
	var out []tickTarget
	for _, s := range specs {
		idx, sub, ok := strings.Cut(s, ":")
		if !ok {
			return nil, errs.Invalid("invalid --tick %q (IIII:SS, hexadecimal)", s)
		}
		i, err := cli.ParseIndex(idx)
		if err != nil {
			return nil, err
		}
		sv, err := strconv.ParseUint(strings.TrimPrefix(sub, "0x"), 16, 8)
		if err != nil {
			return nil, errs.Invalid("invalid --tick %q (IIII:SS, hexadecimal)", s)
		}
		v, err := d.Variable(i, uint8(sv))
		if err != nil {
			return nil, errs.Invalid("--tick %s: no such object in the dictionary", s)
		}
		if !v.DataType.IsInteger() || v.DataType.IsSigned() {
			return nil, errs.Invalid("--tick %s: %s is not an unsigned integer", s, v.DataType)
		}
		mask := uint64(1)<<(v.DataType.Bits()) - 1
		if v.DataType.Bits() == 64 {
			mask = ^uint64(0)
		}
		out = append(out, tickTarget{i, uint8(sv), mask})
	}
	return out, nil
}

// parseEMCY parses --emcy CODE[:REG[:MSEF]] (hexadecimal).
func parseEMCY(s string) (uint16, emcy.ErrorRegister, [5]byte, error) {
	var msef [5]byte
	p := strings.Split(s, ":")
	if len(p) > 3 {
		return 0, 0, msef, errs.Invalid("invalid --emcy %q (CODE[:REG[:MSEF]], hexadecimal)", s)
	}
	code, err := strconv.ParseUint(strings.TrimPrefix(p[0], "0x"), 16, 16)
	if err != nil || code == 0 {
		return 0, 0, msef, errs.Invalid("invalid --emcy code %q (hexadecimal 0001..FFFF)", p[0])
	}
	reg := emcy.ClassRegister(uint16(code))
	if len(p) > 1 {
		r, err := strconv.ParseUint(strings.TrimPrefix(p[1], "0x"), 16, 8)
		if err != nil {
			return 0, 0, msef, errs.Invalid("invalid --emcy register %q (hexadecimal 00..FF)", p[1])
		}
		reg = emcy.ErrorRegister(r)
	}
	if len(p) > 2 {
		b, err := hex.DecodeString(p[2])
		if err != nil || len(b) > 5 {
			return 0, 0, msef, errs.Invalid("invalid --emcy MSEF %q (up to 5 bytes in hexadecimal)", p[2])
		}
		copy(msef[:], b)
	}
	return uint16(code), reg, msef, nil
}

type simReady struct {
	Node    canopen.NodeID `json:"node"`
	State   string         `json:"state"`
	Objects int            `json:"objects"`
	Source  string         `json:"source"`
	Drive   bool           `json:"drive"`
}

type simState struct {
	Timestamp string         `json:"timestamp"`
	Node      canopen.NodeID `json:"node"`
	From      string         `json:"from"`
	To        string         `json:"to"`
	Command   string         `json:"command,omitempty"`
}

type simSummary struct {
	Node  canopen.NodeID `json:"node"`
	State string         `json:"state"`
	Ticks uint64         `json:"ticks"`
	Stats node.Stats     `json:"stats"`
}

func newSimulateCmd(a *app) *cobra.Command {
	var (
		bf                      busFlags
		edsPath                 string
		nodeID                  int
		drive, autoStart        bool
		drivePeriod, tickPeriod time.Duration
		heartbeat               int
		ticks                   []string
		emcySpec                string
		emcyAfter, duration     time.Duration
	)
	c := &cobra.Command{
		Use:   "simulate",
		Short: "Run a simulated CANopen node (from an EDS, optionally a CiA 402 drive)",
		Long: `simulate runs a complete CANopen device (go-canopen's node): NMT slave with
boot-up, SDO server, heartbeat producer and consumer, node guarding, EMCY,
SYNC, TIME and every PDO of its dictionary, built from --eds (an EDS or DCF;
objects a dictionary cannot hold are skipped with a warning) or, without it, a
built-in device with the CiA 301 objects and application objects 0x2000
(counter, UNSIGNED32, mapped in TPDO1, event-driven), 0x2001 (UNSIGNED8,
mapped in RPDO1), 0x2002 (VISIBLE_STRING) and 0x2003 (DOMAIN).

--drive adds a simulated CiA 402 drive (state machine, profile position and
velocity, homing...). --tick increments objects every --tick-period, so that
event-driven TPDOs carry changing data; --emcy raises an emergency
--emcy-after the start. The node runs until --duration or Ctrl-C and prints
its NMT state changes. It transmits like any device: on an interface that is
not virtual it needs --yes. On "virtual" it shares the in-process bus with
nothing else; use vcan0 to talk to it from another canopenctl.`,
		Example: `  canopenctl simulate -i vcan0 --node-id 5
  canopenctl simulate -i vcan0 --eds io401.eds --node-id 5 --tick 6000:01 --auto-start
  canopenctl simulate -i vcan0 --node-id 3 --drive`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if drivePeriod <= 0 || tickPeriod < time.Millisecond {
				return errs.Invalid("--drive-period more than 0, --tick-period at least 1ms")
			}
			if heartbeat < -1 || heartbeat > 0xFFFF {
				return errs.Invalid("--heartbeat %d: 0..65535 ms", heartbeat)
			}
			var d *od.Dictionary
			source := "built-in"
			if edsPath != "" {
				f, err := cli.LoadEDS(edsPath)
				if err != nil {
					return err
				}
				if nodeID == 0 && f.DeviceComissioning != nil {
					nodeID = int(f.DeviceComissioning.NodeID)
				}
				if nodeID < 1 || nodeID > 127 {
					return errs.Invalid("--node-id %d: 1..127 (required unless the DCF has one)", nodeID)
				}
				var diags []string
				dd, ds, err := cli.Dictionary(f, edsPath, canopen.NodeID(nodeID), true)
				if err != nil {
					return err
				}
				for _, x := range ds {
					diags = append(diags, x.String())
				}
				for _, x := range diags {
					a.warn("%s: %s", edsPath, x)
				}
				d, source = dd, edsPath
			} else {
				if nodeID < 1 || nodeID > 127 {
					return errs.Invalid("--node-id %d: 1..127 (required)", nodeID)
				}
				var err error
				if d, err = builtinDictionary(canopen.NodeID(nodeID), drive); err != nil {
					return errs.New(errs.KindInternal, errs.CodeInternalUnexpected, "built-in dictionary: %v", err)
				}
			}
			if heartbeat >= 0 {
				if d.Object(od.IndexProducerHeartbeat) == nil {
					return errs.Invalid("--heartbeat: the dictionary has no 0x1017")
				}
				_ = d.SetUint(od.IndexProducerHeartbeat, 0, uint64(heartbeat))
			}
			var code uint16
			var reg emcy.ErrorRegister
			var msef [5]byte
			if emcySpec != "" {
				var err error
				if code, reg, msef, err = parseEMCY(emcySpec); err != nil {
					return err
				}
			}
			var opts []node.Option
			if drive {
				opts = append(opts, node.WithDrive(cia402.DriveConfig{}, drivePeriod))
			}
			if autoStart {
				opts = append(opts, node.WithAutoStart())
			}
			states := make(chan nmt.StateChange, 256)
			opts = append(opts, node.WithStateHandler(func(sc nmt.StateChange) {
				select {
				case states <- sc:
				default:
				}
			}))
			if err := bf.check("simulate"); err != nil {
				return err
			}
			bus, err := openBus(bf.iface, openOpts{})
			if err != nil {
				return err
			}
			defer func() { _ = bus.Close() }()
			n, err := node.New(bus, d, opts...)
			if err != nil {
				return errs.Invalid("cannot build the node: %v", err)
			}
			tt, err := parseTicks(ticks, n.Dictionary())
			if err != nil {
				_ = n.Close()
				return err
			}
			if err := n.Start(); err != nil {
				return err
			}
			defer func() { _ = n.Close() }()
			st := a.stream()
			if err := st.Record("ready", fmt.Sprintf("node %d running (%s, %d objects), %s", n.NodeID(), source, len(d.Objects()), n.State()),
				simReady{Node: n.NodeID(), State: n.State().String(), Objects: len(d.Objects()), Source: source, Drive: drive}); err != nil {
				return errs.New(errs.KindOutput, errs.CodeOutputFailed, "write output: %w", err)
			}
			ctx, cancel := bounded(cmd.Context(), duration)
			defer cancel()
			var tickC <-chan time.Time
			if len(tt) > 0 {
				tk := time.NewTicker(tickPeriod)
				defer tk.Stop()
				tickC = tk.C
			}
			var emcyC <-chan time.Time
			if emcySpec != "" {
				tm := time.NewTimer(emcyAfter)
				defer tm.Stop()
				emcyC = tm.C
			}
			var nTicks uint64
		loop:
			for {
				select {
				case <-ctx.Done():
					break loop
				case sc := <-states:
					r := simState{Timestamp: stamp(sc.Time), Node: sc.Node, From: sc.From.String(), To: sc.To.String()}
					text := fmt.Sprintf("(%s) node %d %s -> %s", r.Timestamp, sc.Node, sc.From, sc.To)
					if sc.Command != 0 {
						r.Command = sc.Command.String()
						text += " (NMT " + r.Command + ")"
					}
					if err := st.Record("state", text, r); err != nil {
						return errs.New(errs.KindOutput, errs.CodeOutputFailed, "write output: %w", err)
					}
				case <-tickC:
					nTicks++
					dict := n.Dictionary()
					for _, t := range tt {
						v, err := dict.GetUint(t.index, t.sub)
						if err == nil {
							_ = dict.SetUint(t.index, t.sub, (v+1)&t.mask)
						}
					}
				case <-emcyC:
					if err := n.EMCY().Raise(ctx, code, reg, msef); err != nil {
						a.warn("emcy: %v", err)
					}
				}
			}
			sum := simSummary{Node: n.NodeID(), State: n.State().String(), Ticks: nTicks, Stats: n.Stats()}
			if a.format == "json" {
				return st.Record("summary", "", sum)
			}
			s := sum.Stats
			a.warn("node %d: %s; %d SDO requests, %d heartbeats, %d EMCY, %d resets", sum.Node, sum.State, s.SDO.Requests, s.Heartbeat.Sent, s.EMCY.Sent, s.Resets)
			return nil
		},
	}
	bf.add(c, true)
	c.Flags().StringVar(&edsPath, "eds", "", "EDS or DCF of the simulated device (default: a built-in device)")
	c.Flags().IntVar(&nodeID, "node-id", 0, "Node-ID 1..127 (default: the DCF's)")
	c.Flags().BoolVar(&drive, "drive", false, "Simulate a CiA 402 drive (adds its objects where missing)")
	c.Flags().DurationVar(&drivePeriod, "drive-period", time.Millisecond, "Simulation step of the drive")
	c.Flags().BoolVar(&autoStart, "auto-start", false, "Enter operational right after the boot-up (and after every reset)")
	c.Flags().IntVar(&heartbeat, "heartbeat", -1, "Producer heartbeat time 0x1017 in ms (default: the dictionary's)")
	c.Flags().StringArrayVar(&ticks, "tick", nil, "Increment this unsigned object (IIII:SS, hexadecimal) every --tick-period (repeatable)")
	c.Flags().DurationVar(&tickPeriod, "tick-period", 100*time.Millisecond, "Period of --tick")
	c.Flags().StringVar(&emcySpec, "emcy", "", "Raise this emergency: CODE[:REG[:MSEF]] in hexadecimal (e.g. 2310, 5000:81:0102)")
	c.Flags().DurationVar(&emcyAfter, "emcy-after", 0, "Delay of --emcy after the start")
	c.Flags().DurationVar(&duration, "duration", 0, "Stop after this time (0: until Ctrl-C)")
	return c
}
