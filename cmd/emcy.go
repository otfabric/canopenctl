// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/otfabric/canopenctl/internal/cli"
	"github.com/otfabric/canopenctl/internal/errs"
	"github.com/otfabric/go-canopen"
	"github.com/otfabric/go-canopen/dissect"
	"github.com/otfabric/go-canopen/emcy"
	syncobj "github.com/otfabric/go-canopen/sync"
	"github.com/otfabric/go-canopen/timestamp"
	"github.com/spf13/cobra"
)

// emcyRecord is one emergency message of emcy watch.
type emcyRecord struct {
	Timestamp string         `json:"timestamp"`
	Node      canopen.NodeID `json:"node"`
	COBID     string         `json:"cobId"`
	emcyJSON
}

type emcySummary struct {
	Received uint64 `json:"received"`
	Shown    int    `json:"shown"`
	Resets   uint64 `json:"resets"`
	Invalid  uint64 `json:"invalid"`
}

func newEMCYCmd(a *app) *cobra.Command {
	c := &cobra.Command{
		Use:   "emcy",
		Short: "Watch emergency messages",
	}
	var (
		bf       busFlags
		nodes    string
		count    int
		duration time.Duration
	)
	w := &cobra.Command{
		Use:   "watch",
		Short: "Print the emergency messages of the network, decoded (passive)",
		Long: `watch receives the emergency messages (EMCY, 0x081..0x0FF) of every node, or
of --node, and prints each with the meaning of its error code (CiA 301 table
21), the error register bits and the manufacturer-specific field. An error
code 0x0000 is an error reset. It never transmits.`,
		Example: `  canopenctl emcy watch -i can0
  canopenctl emcy watch -i vcan0 --node 5 --count 1 --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			want := map[canopen.NodeID]bool{}
			if nodes != "" {
				list, err := cli.ParseNodeList(nodes)
				if err != nil {
					return err
				}
				for _, n := range list {
					want[n] = true
				}
			}
			if err := bf.checkFD(); err != nil {
				return err
			}
			bus, err := bf.open(true)
			if err != nil {
				return err
			}
			defer func() { _ = bus.Close() }()
			events := make(chan emcy.Event, 1024)
			eopts := []emcy.Option{emcy.WithHandler(func(e emcy.Event) {
				select {
				case events <- e:
				default:
				}
			})}
			if bf.fd {
				eopts = append(eopts, emcy.WithFD(bf.brs))
			}
			cons := emcy.NewConsumer(bus, eopts...)
			if err := cons.Start(); err != nil {
				return err
			}
			defer func() { _ = cons.Close() }()
			ctx, cancel := bounded(cmd.Context(), duration)
			defer cancel()
			st := a.stream()
			sum := emcySummary{}
		loop:
			for count == 0 || sum.Shown < count {
				select {
				case <-ctx.Done():
					break loop
				case e := <-events:
					if len(want) > 0 && !want[e.Node] {
						continue
					}
					r := emcyRecord{Timestamp: stamp(e.Time), Node: e.Node, COBID: cli.Hex(uint64(e.COBID.ID()), 3),
						emcyJSON: *emcyOf(e.Code, uint8(e.Register), e.MSEF[:])}
					text := fmt.Sprintf("(%s) node %d EMCY %s %s, register %s %v, msef %s", r.Timestamp, e.Node, r.Code, r.CodeText, r.Register, r.RegisterNames, r.MSEF)
					if e.IsReset() {
						text = fmt.Sprintf("(%s) node %d EMCY error reset (no error active), register %s", r.Timestamp, e.Node, r.Register)
					}
					if x := e.FD; x != nil {
						r.FD = &dissect.EMCYFD{LogicalDevice: x.LogicalDevice, CiASpec: x.CiASpec, Status: x.Status,
							Milliseconds: x.Time.Ms, Days: x.Time.Days, Time: x.Time.Time()}
						text += fmt.Sprintf(" (FD: logical device %d, CiA %d, status 0x%02X)", x.LogicalDevice, x.CiASpec, x.Status)
					}
					if err := st.Record("emcy", text, r); err != nil {
						return errs.New(errs.KindOutput, errs.CodeOutputFailed, "write output: %w", err)
					}
					sum.Shown++
				}
			}
			s := cons.Stats()
			sum.Received, sum.Resets, sum.Invalid = s.Received, s.Resets, s.Invalid
			if a.format == "json" {
				return st.Record("summary", "", sum)
			}
			a.warn("%d emergency messages (%d shown, %d error resets), %d invalid", sum.Received, sum.Shown, sum.Resets, sum.Invalid)
			return nil
		},
	}
	bf.add(w, false)
	bf.addFD(w, "decode 20-byte CANopen FD emergencies")
	w.Flags().StringVar(&nodes, "node", "", "Only these nodes: 5, 1-10,20")
	w.Flags().IntVarP(&count, "count", "n", 0, "Stop after this many messages (0: no limit)")
	w.Flags().DurationVar(&duration, "duration", 0, "Stop after this time (0: until Ctrl-C)")
	c.AddCommand(w)
	return c
}

// production are the bounds of sync and time.
type production struct {
	count    int
	period   time.Duration
	duration time.Duration
}

func (p *production) add(c *cobra.Command, count int, period time.Duration) {
	c.Flags().IntVarP(&p.count, "count", "n", count, "Send this many messages (0: until --duration)")
	c.Flags().DurationVar(&p.period, "period", period, "Time between two messages (at least 1ms)")
	c.Flags().DurationVar(&p.duration, "duration", 0, "Stop after this time (0: no limit; needed with --count 0)")
}

func (p *production) validate() error {
	if p.count < 0 || p.count > 1000000 {
		return errs.Invalid("--count %d: 0..1000000", p.count)
	}
	if p.count == 0 && p.duration <= 0 {
		return errs.Invalid("--count 0 needs a --duration: production is always bounded")
	}
	if p.count != 1 && p.period < time.Millisecond {
		return errs.Invalid("--period %v: at least 1ms", p.period)
	}
	return nil
}

// run calls send count times (or until the duration), period apart.
func (p *production) run(ctx context.Context, send func(context.Context) error) (int, error) {
	ctx, cancel := bounded(ctx, p.duration)
	defer cancel()
	tick := time.NewTicker(max(p.period, time.Millisecond))
	defer tick.Stop()
	sent := 0
	for p.count == 0 || sent < p.count {
		if err := send(ctx); err != nil {
			if ended(ctx, err) {
				break
			}
			return sent, err
		}
		sent++
		if p.count != 0 && sent == p.count {
			break
		}
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return sent, nil
			}
			return sent, ctx.Err()
		case <-tick.C:
		}
	}
	return sent, nil
}

type syncResult struct {
	Sent     int    `json:"sent"`
	PeriodUs int64  `json:"periodUs"`
	Overflow uint8  `json:"counterOverflow"`
	COBID    string `json:"cobId"`
}

func (r syncResult) Text() string {
	s := fmt.Sprintf("%d SYNC sent on %s, every %v", r.Sent, r.COBID, time.Duration(r.PeriodUs)*time.Microsecond)
	if r.Overflow > 0 {
		s += fmt.Sprintf(", counter 1..%d", r.Overflow)
	}
	return s
}

func newSyncCmd(a *app) *cobra.Command {
	var (
		bf       busFlags
		p        production
		overflow int
		cob      string
	)
	c := &cobra.Command{
		Use:   "sync",
		Short: "Produce SYNC messages (needs --yes off virtual interfaces)",
		Long: `sync sends SYNC messages (CiA 301) every --period, --count times (default 10)
or until --duration. --counter adds the SYNC counter byte running from 1 to
the overflow value (2..240). Synchronous PDOs of the nodes are triggered by
it: production is a state change of the network and needs --yes on a real
interface. Timing is that of a Go ticker (jitter of tens to hundreds of
microseconds): use it for tests and commissioning, not as a precision clock.`,
		Example: `  canopenctl sync -i vcan0 --period 10ms --count 100
  canopenctl sync -i can0 --period 1ms --duration 10s --counter 16 --yes`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := p.validate(); err != nil {
				return err
			}
			if overflow != 0 && (overflow < 2 || overflow > syncobj.MaxOverflow) {
				return errs.Invalid("--counter %d: 0 (no counter) or 2..240", overflow)
			}
			cobID, err := cli.ParseCOBID(cob)
			if err != nil {
				return err
			}
			if err := bf.check("sync"); err != nil {
				return err
			}
			if err := bf.checkFD(); err != nil {
				return err
			}
			bus, err := bf.open(false)
			if err != nil {
				return err
			}
			defer func() { _ = bus.Close() }()
			sopts := []syncobj.Option{syncobj.WithCOBID(cobID), syncobj.WithOverflow(uint8(overflow))}
			if bf.fd {
				sopts = append(sopts, syncobj.WithFD(bf.brs))
			}
			prod, err := syncobj.NewProducer(bus, sopts...)
			if err != nil {
				return errs.Invalid("%v", err)
			}
			defer func() { _ = prod.Close() }()
			sent, err := p.run(cmd.Context(), prod.Send)
			if err != nil {
				return err
			}
			return a.emit("sync", syncResult{Sent: sent, PeriodUs: p.period.Microseconds(), Overflow: uint8(overflow), COBID: cli.Hex(uint64(cobID.ID()), 3)})
		},
	}
	bf.add(c, true)
	bf.addFD(c, "send the SYNC in CAN FD frames")
	p.add(c, 10, 100*time.Millisecond)
	c.Flags().IntVar(&overflow, "counter", 0, "SYNC counter overflow value: 0 (no counter) or 2..240")
	c.Flags().StringVar(&cob, "cob-id", "80", "COB-ID of the SYNC message (hexadecimal)")
	return c
}

type timeResult struct {
	Sent  int    `json:"sent"`
	Time  string `json:"time"`
	Days  uint16 `json:"days"`
	Ms    uint32 `json:"ms"`
	COBID string `json:"cobId"`
}

func (r timeResult) Text() string {
	return fmt.Sprintf("%d TIME message(s) sent on %s: %s (day %d, %d ms)", r.Sent, r.COBID, r.Time, r.Days, r.Ms)
}

func newTimeCmd(a *app) *cobra.Command {
	var (
		bf  busFlags
		p   production
		at  string
		cob string
	)
	c := &cobra.Command{
		Use:   "time",
		Short: "Send the TIME stamp (needs --yes off virtual interfaces)",
		Long: `time sends a TIME message (CiA 301 TIME_OF_DAY: milliseconds after midnight
and days since 1984-01-01, UTC) with the current time, or --at. With --count
it repeats every --period (the current time each time unless --at is given).`,
		Example: `  canopenctl time -i vcan0
  canopenctl time -i can0 --at 2026-10-29T12:00:00Z --yes`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := p.validate(); err != nil {
				return err
			}
			var fixed time.Time
			if at != "" {
				t, err := time.Parse(time.RFC3339Nano, at)
				if err != nil {
					return errs.Invalid("invalid --at %q (RFC 3339, e.g. 2026-10-29T12:00:00Z)", at)
				}
				if _, err := timestamp.FromTime(t); err != nil {
					return errs.Invalid("--at %s: %v", at, err)
				}
				fixed = t
			}
			cobID, err := cli.ParseCOBID(cob)
			if err != nil {
				return err
			}
			if err := bf.check("time"); err != nil {
				return err
			}
			bus, err := openBus(bf.iface, openOpts{})
			if err != nil {
				return err
			}
			defer func() { _ = bus.Close() }()
			prod, err := timestamp.NewProducer(bus, timestamp.WithCOBID(cobID))
			if err != nil {
				return errs.Invalid("%v", err)
			}
			defer func() { _ = prod.Close() }()
			var last time.Time
			sent, err := p.run(cmd.Context(), func(ctx context.Context) error {
				last = fixed
				if last.IsZero() {
					last = time.Now()
				}
				return prod.Send(ctx, last)
			})
			if err != nil {
				return err
			}
			td, _ := timestamp.FromTime(last)
			return a.emit("time", timeResult{Sent: sent, Time: td.Time().Format(time.RFC3339Nano), Days: td.Days, Ms: td.Ms, COBID: cli.Hex(uint64(cobID.ID()), 3)})
		},
	}
	bf.add(c, true)
	p.add(c, 1, time.Second)
	c.Flags().StringVar(&at, "at", "", "Send this time (RFC 3339) instead of the current time")
	c.Flags().StringVar(&cob, "cob-id", "100", "COB-ID of the TIME message (hexadecimal)")
	return c
}
