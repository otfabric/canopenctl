// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/otfabric/canopenctl/internal/cli"
	"github.com/otfabric/canopenctl/internal/errs"
	"github.com/otfabric/go-canbus"
	"github.com/otfabric/go-canopen"
	"github.com/otfabric/go-canopen/dissect"
	"github.com/otfabric/go-canopen/emcy"
	"github.com/spf13/cobra"
)

// services are the --service names and the message kinds they select.
var services = map[string][]dissect.Kind{
	"nmt":       {dissect.KindNMT},
	"sync":      {dissect.KindSYNC},
	"time":      {dissect.KindTIME},
	"emcy":      {dissect.KindEMCY},
	"pdo":       {dissect.KindTPDO, dissect.KindRPDO},
	"tpdo":      {dissect.KindTPDO},
	"rpdo":      {dissect.KindRPDO},
	"sdo":       {dissect.KindSDORequest, dissect.KindSDOResponse},
	"heartbeat": {dissect.KindHeartbeat, dissect.KindBootUp, dissect.KindGuardRequest, dissect.KindGuardResponse},
	"lss":       {dissect.KindLSSMaster, dissect.KindLSSSlave},
	"unknown":   {dissect.KindUnknown},
}

func serviceNames() string {
	var n []string
	for k := range services {
		n = append(n, k)
	}
	sort.Strings(n)
	return strings.Join(n, ", ")
}

// msgFilter selects messages by node and service.
type msgFilter struct {
	nodes map[canopen.NodeID]bool
	kinds map[dissect.Kind]bool
}

func parseMsgFilter(nodes string, svcs []string) (msgFilter, error) {
	var f msgFilter
	if nodes != "" {
		list, err := cli.ParseNodeList(nodes)
		if err != nil {
			return f, err
		}
		f.nodes = map[canopen.NodeID]bool{}
		for _, n := range list {
			f.nodes[n] = true
		}
	}
	for _, s := range svcs {
		for _, name := range strings.Split(s, ",") {
			name = strings.ToLower(strings.TrimSpace(name))
			if name == "" {
				continue
			}
			ks, ok := services[name]
			if !ok {
				return f, errs.Invalid("unknown service %q (%s)", name, serviceNames())
			}
			if f.kinds == nil {
				f.kinds = map[dissect.Kind]bool{}
			}
			for _, k := range ks {
				f.kinds[k] = true
			}
		}
	}
	return f, nil
}

// match reports whether a message passes. A node filter selects the
// messages of those nodes and the messages addressed to them (NMT
// commands to them or to all nodes); SYNC, TIME and LSS carry no node and
// pass it.
func (f msgFilter) match(m *dissect.Message) bool {
	if f.kinds != nil && !f.kinds[m.Kind] {
		return false
	}
	if f.nodes == nil {
		return true
	}
	switch m.Kind {
	case dissect.KindNMT:
		return m.NMT != nil && (m.NMT.Target == 0 || f.nodes[m.NMT.Target])
	case dissect.KindSYNC, dissect.KindTIME, dissect.KindLSSMaster, dissect.KindLSSSlave:
		return true
	default:
		return f.nodes[m.Node]
	}
}

// messageRecord is the JSON form of a decoded CANopen message.
type messageRecord struct {
	Timestamp    string                `json:"timestamp,omitempty"`
	Interface    string                `json:"interface,omitempty"`
	ID           uint32                `json:"id"`
	IDHex        string                `json:"idHex"`
	Extended     bool                  `json:"extended"`
	Remote       bool                  `json:"remote"`
	Len          int                   `json:"len"`
	Data         string                `json:"data"`
	Kind         dissect.Kind          `json:"kind"`
	Node         canopen.NodeID        `json:"node,omitempty"`
	Summary      string                `json:"summary"`
	Error        string                `json:"error,omitempty"`
	NMT          *dissect.NMT          `json:"nmt,omitempty"`
	SYNC         *dissect.SYNC         `json:"sync,omitempty"`
	TIME         *dissect.TIME         `json:"timeStamp,omitempty"`
	EMCY         *emcyJSON             `json:"emcy,omitempty"`
	PDO          *dissect.PDO          `json:"pdo,omitempty"`
	SDO          *dissect.SDO          `json:"sdo,omitempty"`
	ErrorControl *dissect.ErrorControl `json:"errorControl,omitempty"`
	LSS          *dissect.LSS          `json:"lss,omitempty"`
}

// emcyJSON is an emergency message with its descriptions.
type emcyJSON struct {
	Code          string   `json:"code"`
	CodeText      string   `json:"codeText"`
	Register      string   `json:"register"`
	RegisterNames []string `json:"registerNames"`
	MSEF          string   `json:"msef"`
	Reset         bool     `json:"reset,omitempty"`
}

func emcyOf(code uint16, reg uint8, msef []byte) *emcyJSON {
	names := emcy.ErrorRegister(reg).Names()
	if names == nil {
		names = []string{}
	}
	return &emcyJSON{Code: cli.Hex(uint64(code), 4), CodeText: emcy.CodeText(code), Register: cli.Hex(uint64(reg), 2),
		RegisterNames: names, MSEF: hex.EncodeToString(msef), Reset: code == 0}
}

// messageOf builds the record and the text line of a decoded frame.
func messageOf(f *canbus.Frame, m *dissect.Message) (messageRecord, string) {
	r := messageRecord{Interface: f.Interface, ID: uint32(f.ID), Extended: f.IsExtended(), Remote: f.IsRemote(),
		Len: int(f.Len), Data: hex.EncodeToString(f.Payload()), Kind: m.Kind, Node: m.Node, Error: m.Error,
		NMT: m.NMT, SYNC: m.SYNC, TIME: m.TIME, PDO: m.PDO, SDO: m.SDO, ErrorControl: m.ErrorControl, LSS: m.LSS}
	r.IDHex = fmt.Sprintf("%03X", r.ID)
	if r.Extended {
		r.IDHex = fmt.Sprintf("%08X", r.ID)
	}
	if !f.Timestamp.IsZero() {
		r.Timestamp = stamp(f.Timestamp)
	}
	r.Summary = m.String()
	if m.EMCY != nil {
		r.EMCY = emcyOf(m.EMCY.Code, m.EMCY.Register, m.EMCY.MSEF)
		if !strings.Contains(r.Summary, r.EMCY.CodeText) {
			r.Summary += " (" + r.EMCY.CodeText + ")"
		}
	}
	ts := "(-)"
	if r.Timestamp != "" {
		ts = "(" + r.Timestamp + ")"
	}
	iface := f.Interface
	if iface == "" {
		iface = "-"
	}
	text := fmt.Sprintf("%s %s %s  %s", ts, iface, f.String(), r.Summary)
	if m.Error != "" && !strings.Contains(r.Summary, m.Error) {
		text += "  [" + m.Error + "]"
	}
	return r, text
}

// timestamp formats a time as seconds with microseconds, as candump.
func stamp(t time.Time) string {
	return fmt.Sprintf("%d.%06d", t.Unix(), t.Nanosecond()/1000)
}

// monitorSummary ends monitor and decode.
type monitorSummary struct {
	Frames    uint64            `json:"frames"`
	Shown     uint64            `json:"shown"`
	Malformed uint64            `json:"malformed"`
	Dropped   uint64            `json:"dropped"`
	Kinds     map[string]uint64 `json:"kinds"`
}

func (s monitorSummary) line() string {
	return fmt.Sprintf("%d frames, %d shown, %d malformed, %d dropped", s.Frames, s.Shown, s.Malformed, s.Dropped)
}

// dissector decodes frames and writes the records that pass the filter.
type dissector struct {
	a       *app
	tracker *dissect.Tracker
	filter  msgFilter
	sum     monitorSummary
}

func newDissector(a *app, f msgFilter) *dissector {
	return &dissector{a: a, tracker: dissect.NewTracker(), filter: f, sum: monitorSummary{Kinds: map[string]uint64{}}}
}

// frame decodes one frame and writes it when it passes; shown reports
// whether it did.
func (d *dissector) frame(f *canbus.Frame) (shown bool, err error) {
	d.sum.Frames++
	if f.IsError() || f.IsFD() {
		return false, nil // CANopen runs on classical CAN data and remote frames
	}
	m := d.tracker.Decode(f)
	if m.Error != "" {
		d.sum.Malformed++
	}
	if !d.filter.match(&m) {
		return false, nil
	}
	d.sum.Shown++
	d.sum.Kinds[string(m.Kind)]++
	r, text := messageOf(f, &m)
	if err := d.a.stream().Record("message", text, r); err != nil {
		return false, errs.New(errs.KindOutput, errs.CodeOutputFailed, "write output: %w", err)
	}
	return true, nil
}

func (d *dissector) finish() error {
	if d.a.format == "json" {
		if err := d.a.stream().Record("summary", "", d.sum); err != nil {
			return errs.New(errs.KindOutput, errs.CodeOutputFailed, "write output: %w", err)
		}
	} else {
		d.a.warn("%s", d.sum.line())
	}
	return nil
}

func addMsgFilterFlags(c *cobra.Command, nodes *string, svcs *[]string) {
	c.Flags().StringVar(nodes, "node", "", "Only messages of (or addressed to) these nodes: 5, 1-10,20")
	c.Flags().StringArrayVar(svcs, "service", nil, "Only these services (repeatable, comma-separated): "+serviceNames())
}

func newMonitorCmd(a *app) *cobra.Command {
	var (
		bf       busFlags
		nodes    string
		svcs     []string
		count    int
		duration time.Duration
	)
	c := &cobra.Command{
		Use:   "monitor",
		Short: "Decode live traffic as CANopen messages (passive)",
		Long: `monitor receives frames and decodes them as CANopen messages by the predefined
connection set: NMT, SYNC, TIME, EMCY, PDOs, SDO requests and responses (with
segmented and block transfers followed, so segments name their object),
heartbeat, boot-up, node guarding and LSS. It never transmits; slcan adapters
are opened listen-only. --node and --service select messages; JSON output is
JSON Lines, one record per message and a summary at the end.`,
		Example: `  canopenctl monitor -i can0
  canopenctl monitor -i vcan0 --node 5 --service sdo,emcy --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			filt, err := parseMsgFilter(nodes, svcs)
			if err != nil {
				return err
			}
			if count < 0 {
				return errs.Invalid("--count %d: 0 or more", count)
			}
			bus, err := openBus(bf.iface, openOpts{passive: true})
			if err != nil {
				return err
			}
			defer func() { _ = bus.Close() }()
			sub, err := bus.Subscribe(canbus.WithQueueSize(65536))
			if err != nil {
				return err
			}
			ctx, cancel := bounded(cmd.Context(), duration)
			defer cancel()
			d := newDissector(a, filt)
			var shown int
			for count == 0 || shown < count {
				f, err := sub.Receive(ctx)
				if err != nil {
					if ended(ctx, err) {
						break
					}
					return err
				}
				ok, err := d.frame(&f)
				if err != nil {
					return err
				}
				if ok {
					shown++
				}
			}
			d.sum.Dropped = bus.Stats().Dropped + sub.Dropped()
			return d.finish()
		},
	}
	bf.add(c, false)
	addMsgFilterFlags(c, &nodes, &svcs)
	c.Flags().IntVarP(&count, "count", "n", 0, "Stop after this many messages shown (0: no limit)")
	c.Flags().DurationVar(&duration, "duration", 0, "Stop after this time (0: until Ctrl-C)")
	return c
}

func newDecodeCmd(a *app) *cobra.Command {
	var (
		nodes           string
		svcs            []string
		ignoreMalformed bool
	)
	c := &cobra.Command{
		Use:   "decode <trace>",
		Short: "Decode a trace file as CANopen messages",
		Long: `decode reads a trace (candump log, JSON Lines, pcap, pcapng, ASC, BLF or PEAK
TRC; the format is detected from the content; "-" reads stdin) and decodes its
frames as CANopen messages, as monitor does. Malformed CANopen messages (a
wrong length, say) are shown with the problem and make the exit code 1.`,
		Example: `  canopenctl decode trip.log
  canopenctl decode capture.pcapng --node 5 --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			filt, err := parseMsgFilter(nodes, svcs)
			if err != nil {
				return err
			}
			r, closeFn, err := cli.OpenTrace(args[0], a.in)
			if err != nil {
				return err
			}
			defer closeFn()
			d := newDissector(a, filt)
			for {
				var f canbus.Frame
				if err := r.Read(&f); err != nil {
					if errors.Is(err, io.EOF) {
						break
					}
					return &errs.Error{Kind: errs.KindInvalidInput, Code: errs.CodeInvalidCapture, Message: fmt.Sprintf("%s: %v", args[0], err), Cause: err}
				}
				if _, err := d.frame(&f); err != nil {
					return err
				}
			}
			if err := d.finish(); err != nil {
				return err
			}
			if d.sum.Malformed > 0 && !ignoreMalformed {
				return errs.ErrFindings
			}
			return nil
		},
	}
	addMsgFilterFlags(c, &nodes, &svcs)
	c.Flags().BoolVar(&ignoreMalformed, "ignore-malformed", false, "Exit 0 even when malformed messages were found")
	return c
}
