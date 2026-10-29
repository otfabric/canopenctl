// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/otfabric/canopenctl/internal/cli"
	"github.com/otfabric/canopenctl/internal/errs"
	"github.com/otfabric/go-canopen"
	"github.com/otfabric/go-canopen/master"
	"github.com/otfabric/go-canopen/nmt"
	"github.com/otfabric/go-canopen/sdo"
	"github.com/otfabric/go-canopen/usdo"
	"github.com/spf13/cobra"
)

// nmtCommands are the nmt command names.
var nmtCommands = map[string]canopen.NMTCommand{
	"start": canopen.NMTStart, "stop": canopen.NMTStop,
	"preop": canopen.NMTEnterPreOperational, "pre-operational": canopen.NMTEnterPreOperational,
	"reset-node": canopen.NMTResetNode, "reset-comm": canopen.NMTResetCommunication,
	"reset-communication": canopen.NMTResetCommunication,
}

// targetState is the state an NMT command leads to.
func targetState(c canopen.NMTCommand) canopen.NMTState {
	switch c {
	case canopen.NMTStart:
		return canopen.StateOperational
	case canopen.NMTStop:
		return canopen.StateStopped
	case canopen.NMTEnterPreOperational:
		return canopen.StatePreOperational
	default:
		return canopen.StateInitialising // a reset: boot-up
	}
}

type nmtResult struct {
	Node      canopen.NodeID `json:"node"`
	Command   string         `json:"command"`
	Waited    bool           `json:"waited"`
	State     string         `json:"state,omitempty"`
	ElapsedMs int64          `json:"elapsedMs,omitempty"`
}

func (r nmtResult) Text() string {
	target := "all nodes"
	if r.Node != 0 {
		target = "node " + strconv.Itoa(int(r.Node))
	}
	s := fmt.Sprintf("NMT %s sent to %s", r.Command, target)
	if r.Waited {
		s += fmt.Sprintf("; node reports %s after %d ms", r.State, r.ElapsedMs)
	}
	return s
}

func newNMTCmd(a *app) *cobra.Command {
	var (
		bf      busFlags
		wait    bool
		timeout time.Duration
	)
	c := &cobra.Command{
		Use:   "nmt <start|stop|preop|reset-node|reset-comm> <node|all>",
		Short: "Send an NMT command to a node or to all nodes (needs --yes off virtual interfaces)",
		Long: `nmt sends an NMT module control command (CiA 301): start (operational), stop
(stopped), preop (pre-operational), reset-node (reset application) or
reset-comm (reset communication), to one node (1..127) or to all (all, 0).
--wait waits until the node confirms: its heartbeat reports the new state
(start, stop, preop; the node must produce heartbeats), or its boot-up
message arrives (resets); not with all.`,
		Example: `  canopenctl nmt start all -i vcan0
  canopenctl nmt reset-node 5 --wait -i can0 --yes`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			nc, ok := nmtCommands[strings.ToLower(args[0])]
			if !ok {
				return errs.Invalid("unknown NMT command %q (start, stop, preop, reset-node, reset-comm)", args[0])
			}
			node, err := cli.ParseNode(args[1], true)
			if err != nil {
				return err
			}
			if wait && node == 0 {
				return errs.Invalid("--wait needs one node, not all")
			}
			if timeout <= 0 {
				return errs.Invalid("--timeout %v: more than 0", timeout)
			}
			if err := bf.checkFD(); err != nil {
				return err
			}
			if err := bf.check("nmt " + args[0]); err != nil {
				return err
			}
			bus, err := bf.open(false)
			if err != nil {
				return err
			}
			defer func() { _ = bus.Close() }()
			m, err := master.New(bus, bf.masterOptions()...)
			if err != nil {
				return err
			}
			defer func() { _ = m.Close() }()
			if wait {
				if err := m.Start(); err != nil {
					return err
				}
			}
			since := time.Now()
			if err := m.NMT(cmd.Context(), node, nc); err != nil {
				return err
			}
			r := nmtResult{Node: node, Command: nc.String()}
			if wait {
				ctx, cancel := bounded(cmd.Context(), timeout)
				defer cancel()
				st := targetState(nc)
				if st == canopen.StateInitialising {
					err = m.WaitBootUp(ctx, node, since)
				} else {
					err = m.WaitState(ctx, node, st)
				}
				if err != nil {
					if errors.Is(err, context.DeadlineExceeded) && cmd.Context().Err() == nil {
						return errs.New(errs.KindTimeout, errs.CodeNoResponse, "node %d did not report %s within %v", node, stateName(st), timeout)
					}
					return err
				}
				r.Waited, r.State, r.ElapsedMs = true, stateName(st), time.Since(since).Milliseconds()
			}
			return a.emit("nmt", r)
		},
	}
	bf.add(c, true)
	bf.addFD(c, "send the command in a CAN FD frame (--wait: FD heartbeats)")
	c.Flags().BoolVar(&wait, "wait", false, "Wait for the node's heartbeat (or boot-up) to confirm")
	c.Flags().DurationVar(&timeout, "timeout", 5*time.Second, "How long --wait waits")
	return c
}

func stateName(s canopen.NMTState) string {
	if s == canopen.StateInitialising {
		return "boot-up"
	}
	return s.String()
}

// heartbeatRecord is one event of heartbeat watch.
type heartbeatRecord struct {
	Timestamp string         `json:"timestamp"`
	Node      canopen.NodeID `json:"node"`
	Event     string         `json:"event"`
	State     string         `json:"state"`
	Previous  string         `json:"previous"`
}

type heartbeatNode struct {
	Node      canopen.NodeID `json:"node"`
	State     string         `json:"state"`
	BootUps   uint64         `json:"bootUps"`
	Lost      bool           `json:"lost"`
	LastSeen  string         `json:"lastSeen,omitempty"`
	TimeoutMs int64          `json:"timeoutMs,omitempty"`
}

type heartbeatSummary struct {
	Events   int             `json:"events"`
	Timeouts uint64          `json:"timeouts"`
	BootUps  uint64          `json:"bootUps"`
	Nodes    []heartbeatNode `json:"nodes"`
}

func newHeartbeatCmd(a *app) *cobra.Command {
	c := &cobra.Command{
		Use:   "heartbeat",
		Short: "Watch the heartbeats of the network",
	}
	var (
		bf        busFlags
		nodes     string
		supervise time.Duration
		count     int
		duration  time.Duration
	)
	w := &cobra.Command{
		Use:   "watch",
		Short: "Report boot-ups, NMT state changes and heartbeat timeouts (passive)",
		Long: `watch receives the NMT error control messages (heartbeat, boot-up, node
guarding responses) of the network and reports each node's boot-ups and state
changes. With --supervise every node (or those of --node) is supervised from
its first heartbeat: a node silent for longer is reported lost once, and
again when it resumes. It never transmits. At the end a summary lists the
nodes; a heartbeat timeout during the watch makes the exit code 1.`,
		Example: `  canopenctl heartbeat watch -i can0 --supervise 1500ms --duration 1m
  canopenctl heartbeat watch -i vcan0 --node 5,6 --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var list []canopen.NodeID
			var err error
			if nodes != "" {
				if list, err = cli.ParseNodeList(nodes); err != nil {
					return err
				}
			}
			if supervise < 0 || supervise > 65535*time.Millisecond {
				return errs.Invalid("--supervise %v: 0..65535ms", supervise)
			}
			want := map[canopen.NodeID]bool{}
			for _, n := range list {
				want[n] = true
			}
			if err := bf.checkFD(); err != nil {
				return err
			}
			bus, err := bf.open(true)
			if err != nil {
				return err
			}
			defer func() { _ = bus.Close() }()
			events := make(chan nmt.HeartbeatEvent, 1024)
			hopts := []nmt.Option{nmt.WithHeartbeatHandler(func(e nmt.HeartbeatEvent) {
				select {
				case events <- e:
				default:
				}
			})}
			if bf.fd {
				hopts = append(hopts, nmt.WithFD(bf.brs))
			}
			hc, err := nmt.NewHeartbeatConsumer(bus, hopts...)
			if err != nil {
				return err
			}
			if supervise > 0 {
				targets := list
				if len(targets) == 0 {
					for n := canopen.MinNodeID; n <= canopen.MaxNodeID; n++ {
						targets = append(targets, n)
					}
				}
				for _, n := range targets {
					if err := hc.Monitor(n, supervise); err != nil {
						return err
					}
				}
			}
			if err := hc.Start(); err != nil {
				return err
			}
			defer func() { _ = hc.Close() }()
			ctx, cancel := bounded(cmd.Context(), duration)
			defer cancel()
			st := a.stream()
			sum := heartbeatSummary{Nodes: []heartbeatNode{}}
		loop:
			for count == 0 || sum.Events < count {
				select {
				case <-ctx.Done():
					break loop
				case e := <-events:
					if len(want) > 0 && !want[e.Node] {
						continue
					}
					r := heartbeatRecord{Timestamp: stamp(e.Time), Node: e.Node, Event: eventName(e.Kind), State: e.State.String(), Previous: e.Previous.String()}
					if err := st.Record("heartbeat", heartbeatText(e, supervise), r); err != nil {
						return errs.New(errs.KindOutput, errs.CodeOutputFailed, "write output: %w", err)
					}
					sum.Events++
					switch e.Kind {
					case nmt.EventTimeout:
						sum.Timeouts++
					case nmt.EventBootUp:
						sum.BootUps++
					default:
					}
				}
			}
			for _, s := range hc.Nodes() {
				if len(want) > 0 && !want[s.Node] {
					continue
				}
				if s.LastSeen.IsZero() && !s.Lost {
					continue // configured, never heard
				}
				n := heartbeatNode{Node: s.Node, State: s.State.String(), BootUps: s.BootUps, Lost: s.Lost, TimeoutMs: s.Timeout.Milliseconds()}
				if !s.LastSeen.IsZero() {
					n.LastSeen = stamp(s.LastSeen)
				}
				sum.Nodes = append(sum.Nodes, n)
			}
			if a.format == "json" {
				if err := st.Record("summary", "", sum); err != nil {
					return errs.New(errs.KindOutput, errs.CodeOutputFailed, "write output: %w", err)
				}
			} else {
				for _, n := range sum.Nodes {
					lost := ""
					if n.Lost {
						lost = " (lost)"
					}
					a.warn("node %d: %s%s, %d boot-ups", n.Node, n.State, lost, n.BootUps)
				}
				a.warn("%d events, %d boot-ups, %d timeouts", sum.Events, sum.BootUps, sum.Timeouts)
			}
			if sum.Timeouts > 0 {
				return errs.ErrFindings
			}
			return nil
		},
	}
	bf.add(w, false)
	bf.addFD(w, "accept CAN FD heartbeats")
	w.Flags().StringVar(&nodes, "node", "", "Only these nodes: 5, 1-10,20")
	w.Flags().DurationVar(&supervise, "supervise", 0, "Consumer heartbeat time: report a node lost after this silence (0: no supervision)")
	w.Flags().IntVarP(&count, "count", "n", 0, "Stop after this many events (0: no limit)")
	w.Flags().DurationVar(&duration, "duration", 0, "Stop after this time (0: until Ctrl-C)")
	c.AddCommand(w)
	return c
}

func eventName(k nmt.HeartbeatEventKind) string {
	switch k {
	case nmt.EventBootUp:
		return "boot-up"
	case nmt.EventStateChange:
		return "state-change"
	case nmt.EventTimeout:
		return "timeout"
	case nmt.EventResumed:
		return "resumed"
	default:
		return k.String()
	}
}

func heartbeatText(e nmt.HeartbeatEvent, supervise time.Duration) string {
	ts := "(" + stamp(e.Time) + ")"
	switch e.Kind {
	case nmt.EventBootUp:
		return fmt.Sprintf("%s node %d boot-up", ts, e.Node)
	case nmt.EventStateChange:
		return fmt.Sprintf("%s node %d %s -> %s", ts, e.Node, e.Previous, e.State)
	case nmt.EventTimeout:
		return fmt.Sprintf("%s node %d heartbeat lost (no heartbeat for %v)", ts, e.Node, supervise)
	case nmt.EventResumed:
		return fmt.Sprintf("%s node %d heartbeat resumed", ts, e.Node)
	default:
		return fmt.Sprintf("%s node %d %s", ts, e.Node, e.Kind)
	}
}

// profiles names the device profiles of the low 16 bits of 0x1000.
var profiles = map[uint32]string{
	301: "CiA 301 (no profile)", 401: "CiA 401 generic I/O", 402: "CiA 402 drives and motion control", 404: "CiA 404 measuring devices",
	405: "CiA 405 IEC 61131-3 programmable devices", 406: "CiA 406 encoders", 408: "CiA 408 hydraulic drives",
	410: "CiA 410 inclinometers", 413: "CiA 413 truck gateways", 415: "CiA 415 road construction", 417: "CiA 417 lift control",
	418: "CiA 418 battery modules", 419: "CiA 419 battery chargers", 443: "CiA 443 SIIS level-2 devices",
	447: "CiA 447 special-purpose car add-on devices", 454: "CiA 454 energy management",
}

func profileName(devType uint32) string {
	p := devType & 0xFFFF
	if p == 0 {
		return "none"
	}
	if n, ok := profiles[p]; ok {
		return n
	}
	return "CiA " + strconv.Itoa(int(p))
}

type scanNode struct {
	Node         canopen.NodeID  `json:"node"`
	DeviceType   string          `json:"deviceType"`
	Profile      string          `json:"profile"`
	VendorID     string          `json:"vendorId"`
	ProductCode  string          `json:"productCode"`
	Revision     string          `json:"revision"`
	SerialNumber string          `json:"serialNumber"`
	Identity     master.Identity `json:"identity"`
}

type scanResult struct {
	Scanned int        `json:"scanned"`
	Nodes   []scanNode `json:"nodes"`
	Errors  []string   `json:"errors"`
}

func (r scanResult) Text() string {
	var b strings.Builder
	for _, n := range r.Nodes {
		fmt.Fprintf(&b, "node %3d  device type %s (%s)  vendor %s  product %s  revision %s  serial %s\n",
			n.Node, n.DeviceType, n.Profile, n.VendorID, n.ProductCode, n.Revision, n.SerialNumber)
	}
	for _, e := range r.Errors {
		fmt.Fprintf(&b, "error: %s\n", e)
	}
	fmt.Fprintf(&b, "%d of %d node-IDs answered", len(r.Nodes), r.Scanned)
	return b.String()
}

func (r scanResult) Headers() []string {
	return []string{"NODE", "DEVICE-TYPE", "PROFILE", "VENDOR", "PRODUCT", "REVISION", "SERIAL"}
}

func (r scanResult) Rows() [][]string {
	var rows [][]string
	for _, n := range r.Nodes {
		rows = append(rows, []string{strconv.Itoa(int(n.Node)), n.DeviceType, n.Profile, n.VendorID, n.ProductCode, n.Revision, n.SerialNumber})
	}
	return rows
}

func newScanCmd(a *app) *cobra.Command {
	var (
		bf       busFlags
		nodes    string
		timeout  time.Duration
		parallel int
	)
	c := &cobra.Command{
		Use:   "scan",
		Short: "Find the nodes of a network: device type and identity of each (SDO reads)",
		Long: `scan reads the device type (0x1000) and the identity (0x1018: vendor-ID,
product code, revision, serial number) of every node-ID 1..127 (or --nodes)
by SDO, --parallel at a time with a short --timeout, and lists the nodes that
answered. It only sends SDO read requests (one per node-ID and object): it
needs no --yes. A node that answers 0x1000 but fails on 0x1018 is listed under
errors and makes the exit code 1.`,
		Example: `  canopenctl scan -i can0
  canopenctl scan -i vcan0 --nodes 1-16 --format table`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			list, err := cli.ParseNodeList(nodes)
			if err != nil {
				return err
			}
			if timeout <= 0 || timeout > 10*time.Second {
				return errs.Invalid("--timeout %v: 1ms..10s", timeout)
			}
			if parallel < 1 || parallel > 127 {
				return errs.Invalid("--parallel %d: 1..127", parallel)
			}
			if err := bf.checkFD(); err != nil {
				return err
			}
			bus, err := bf.open(false)
			if err != nil {
				return err
			}
			defer func() { _ = bus.Close() }()
			mopts := []master.Option{master.WithScanOptions(sdo.WithTimeout(timeout)), master.WithScanParallelism(parallel)}
			if bf.fd {
				mopts = append(mopts, master.WithFD(bf.brs), master.WithUSDOClient(canopen.MaxNodeID, usdo.WithTimeout(timeout)))
			}
			m, err := master.New(bus, mopts...)
			if err != nil {
				return err
			}
			defer func() { _ = m.Close() }()
			found, scanErr := m.Scan(cmd.Context(), list)
			if cmd.Context().Err() != nil {
				return cmd.Context().Err()
			}
			r := scanResult{Scanned: len(list), Nodes: []scanNode{}, Errors: []string{}}
			for _, n := range found {
				id := n.Identity
				r.Nodes = append(r.Nodes, scanNode{Node: n.Node, DeviceType: cli.Hex(uint64(n.DeviceType), 8), Profile: profileName(n.DeviceType),
					VendorID: cli.Hex(uint64(id.VendorID), 8), ProductCode: cli.Hex(uint64(id.ProductCode), 8),
					Revision: cli.Hex(uint64(id.Revision), 8), SerialNumber: cli.Hex(uint64(id.SerialNumber), 8), Identity: id})
			}
			if scanErr != nil {
				for _, l := range strings.Split(scanErr.Error(), "\n") {
					if l = strings.TrimSpace(l); l != "" {
						r.Errors = append(r.Errors, l)
					}
				}
			}
			if err := a.emit("scan", r); err != nil {
				return err
			}
			if len(r.Errors) > 0 {
				return errs.ErrFindings
			}
			return nil
		},
	}
	bf.add(c, false)
	bf.addFD(c, "read over USDO instead of SDO")
	c.Flags().StringVar(&nodes, "nodes", "1-127", "Node-IDs to scan: 1-127, 1-10,20")
	c.Flags().DurationVar(&timeout, "timeout", 100*time.Millisecond, "SDO response timeout per node")
	c.Flags().IntVar(&parallel, "parallel", 16, "Nodes queried at once")
	return c
}
