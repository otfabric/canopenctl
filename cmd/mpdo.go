// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/otfabric/canopenctl/internal/cli"
	"github.com/otfabric/canopenctl/internal/errs"
	"github.com/otfabric/go-canbus"
	"github.com/otfabric/go-canopen"
	"github.com/otfabric/go-canopen/master"
	"github.com/otfabric/go-canopen/mpdo"
	"github.com/spf13/cobra"
)

func newMPDOCmd(a *app) *cobra.Command {
	c := &cobra.Command{
		Use:   "mpdo",
		Short: "Multiplexed PDOs (CiA 301): configure a node's MPDO and its lists, watch MPDO traffic",
		Long: `An MPDO carries one object per message: in destination address mode (DAM) the
producer writes an object of a consumer (or of all), in source address mode
(SAM) it publishes one of its own objects, listed in its object scanner list
(0x1FA0..), which consumers map to local objects with their object
dispatcher list (0x1FD0..). The mpdo commands configure this over SDO (or
USDO with --fd) and watch the traffic.`,
	}
	c.AddCommand(newMPDOConfigureCmd(a), newMPDOListCmd(a, true), newMPDOListCmd(a, false), newMPDOWatchCmd(a))
	return c
}

// pdoSelect parses --tpdo/--rpdo: exactly one.
func pdoSelect(tpdo, rpdo int) (transmit bool, n int, err error) {
	if (tpdo == 0) == (rpdo == 0) {
		return false, 0, errs.Invalid("give --tpdo or --rpdo, not both or neither")
	}
	transmit, n = tpdo != 0, tpdo+rpdo
	if n < 1 || n > 512 {
		return false, 0, errs.Invalid("PDO number %d: 1..512", n)
	}
	return transmit, n, nil
}

type mpdoConfigResult struct {
	Node        canopen.NodeID `json:"node"`
	Kind        string         `json:"kind"`
	PDO         int            `json:"pdo"`
	Mode        string         `json:"mode"`
	COBID       string         `json:"cobId"`
	InhibitTime uint16         `json:"inhibitTime"`
}

func (r mpdoConfigResult) Text() string {
	return fmt.Sprintf("node %d %s%d is an MPDO, %s, COB-ID %s", r.Node, strings.ToUpper(r.Kind), r.PDO, r.Mode, r.COBID)
}

func newMPDOConfigureCmd(a *app) *cobra.Command {
	var (
		bf         busFlags
		sf         sdoFlags
		tpdo, rpdo int
		mode, cob  string
		inhibit    int
	)
	c := &cobra.Command{
		Use:   "configure <node>",
		Short: "Make a TPDO or RPDO of a node a multiplexed PDO (needs --yes off virtual interfaces)",
		Long: `configure makes TPDO or RPDO n of a node an MPDO in destination (--mode dam) or
source address mode (--mode sam), on --cob-id, event-driven (transmission
type 254), with the CiA 301 configuration sequence (invalidate, mapping
sub-index 0 = FEh/FFh, valid). A SAM producer also needs an object scanner
list and its consumers an object dispatcher list ("mpdo scanner", "mpdo
dispatcher"). The device refuses with an abort (exit 11) when it has no MPDO
support.`,
		Example: `  canopenctl mpdo configure 5 --tpdo 1 --mode dam --cob-id 0x185 -i vcan0
  canopenctl mpdo configure 6 --rpdo 2 --mode sam --cob-id 0x285 -i can0 --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			node, err := cli.ParseNode(args[0], false)
			if err != nil {
				return err
			}
			transmit, n, err := pdoSelect(tpdo, rpdo)
			if err != nil {
				return err
			}
			var m uint8
			switch strings.ToLower(mode) {
			case "sam":
				m = mpdo.MappingSAM
			case "dam":
				m = mpdo.MappingDAM
			default:
				return errs.Invalid("--mode %q: sam or dam", mode)
			}
			cobID, err := cli.ParseCOBID(cob)
			if err != nil {
				return err
			}
			if inhibit < 0 || inhibit > 0xFFFF {
				return errs.Invalid("--inhibit %d: 0..65535 (100 µs)", inhibit)
			}
			if err := sf.validate(); err != nil {
				return err
			}
			if err := bf.checkFD(); err != nil {
				return err
			}
			if err := bf.check("mpdo configure"); err != nil {
				return err
			}
			bus, err := bf.open(false)
			if err != nil {
				return err
			}
			defer func() { _ = bus.Close() }()
			mst, err := master.New(bus, sf.masterOptions(&bf)...)
			if err != nil {
				return err
			}
			defer func() { _ = mst.Close() }()
			if err := mst.ConfigureMPDO(cmd.Context(), node, transmit, n, m, cobID, uint16(inhibit)); err != nil {
				return err
			}
			return a.emit("mpdo configure", mpdoConfigResult{Node: node, Kind: pdoKind(transmit), PDO: n, Mode: strings.ToUpper(mode),
				COBID: cli.Hex(uint64(cobID.ID()), 3), InhibitTime: uint16(inhibit)})
		},
	}
	bf.add(c, true)
	bf.addFD(c, "write over USDO instead of SDO")
	sf.add(c, false)
	sf.addFD(c)
	c.Flags().IntVar(&tpdo, "tpdo", 0, "Make this TPDO an MPDO producer (1..512)")
	c.Flags().IntVar(&rpdo, "rpdo", 0, "Make this RPDO an MPDO consumer (1..512)")
	c.Flags().StringVar(&mode, "mode", "", "Address mode: dam (destination) or sam (source) (required)")
	c.Flags().StringVar(&cob, "cob-id", "", "COB-ID of the MPDO (hexadecimal, required)")
	c.Flags().IntVar(&inhibit, "inhibit", 0, "Inhibit time of a TPDO in 100 µs")
	_ = c.MarkFlagRequired("mode")
	_ = c.MarkFlagRequired("cob-id")
	return c
}

type mpdoListResult struct {
	Node       canopen.NodeID         `json:"node"`
	List       int                    `json:"list"`
	Index      string                 `json:"index"`
	Scanner    []mpdo.ScannerEntry    `json:"scanner,omitempty"`
	Dispatcher []mpdo.DispatcherEntry `json:"dispatcher,omitempty"`
	Entries    []string               `json:"entries"`
	kind       string
}

func (r mpdoListResult) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "node %d %s list %d (%s): %d entries written", r.Node, r.kind, r.List, r.Index, len(r.Entries))
	for _, e := range r.Entries {
		b.WriteString("\n  " + e)
	}
	return b.String()
}

func newMPDOListCmd(a *app, scanner bool) *cobra.Command {
	var (
		bf      busFlags
		sf      sdoFlags
		list    int
		entries []string
	)
	use, kind, first, format := "dispatcher", "object dispatcher", mpdo.IndexDispatcher, "NODE:IIII:SS[+n]=IIII:SS"
	long := `dispatcher writes object dispatcher list --list (0x1FD0 + list - 1) of a node,
a consumer of SAM-MPDOs: each --entry NODE:IIII:SS[+n]=IIII:SS maps object
IIII:SS (and n-1 more sub-indices) of producer NODE to a local object. The
entries go to sub-indices 1.., then their number to sub-index 0 (skipped
when the list has a fixed size).`
	example := `  canopenctl mpdo dispatcher 6 --list 1 --entry 5:2000:00=2100:00 --entry 5:2001:01+4=2101:01 -i vcan0`
	if scanner {
		use, kind, first, format = "scanner", "object scanner", mpdo.IndexScanner, "IIII:SS[+n]"
		long = `scanner writes object scanner list --list (0x1FA0 + list - 1) of a node, a
producer of SAM-MPDOs: each --entry IIII:SS[+n] allows the producer to send
object IIII:SS (and n-1 more sub-indices) in SAM-MPDOs. The entries go to
sub-indices 1.., then their number to sub-index 0 (skipped when the list has
a fixed size).`
		example = `  canopenctl mpdo scanner 5 --list 1 --entry 2000:00 --entry 2001:01+4 -i vcan0`
	}
	c := &cobra.Command{
		Use:     use + " <node>",
		Short:   "Write the " + kind + " list of a node (needs --yes off virtual interfaces)",
		Long:    long,
		Example: example,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			node, err := cli.ParseNode(args[0], false)
			if err != nil {
				return err
			}
			if list < 1 || list > mpdo.MaxLists {
				return errs.Invalid("--list %d: 1..%d", list, mpdo.MaxLists)
			}
			if len(entries) > mpdo.MaxListEntries {
				return errs.Invalid("%d entries: at most %d", len(entries), mpdo.MaxListEntries)
			}
			r := mpdoListResult{Node: node, List: list, Index: cli.Hex(uint64(first)+uint64(list-1), 4), Entries: []string{}, kind: kind}
			var se []mpdo.ScannerEntry
			var de []mpdo.DispatcherEntry
			for _, s := range entries {
				for _, x := range strings.Split(s, ",") {
					if x = strings.TrimSpace(x); x == "" {
						continue
					}
					if scanner {
						e, err := cli.ParseScannerEntry(x)
						if err != nil {
							return err
						}
						se = append(se, e)
						r.Entries = append(r.Entries, e.String())
					} else {
						e, err := cli.ParseDispatcherEntry(x)
						if err != nil {
							return err
						}
						de = append(de, e)
						r.Entries = append(r.Entries, e.String())
					}
				}
			}
			if err := sf.validate(); err != nil {
				return err
			}
			if err := bf.checkFD(); err != nil {
				return err
			}
			if err := bf.check("mpdo " + use); err != nil {
				return err
			}
			bus, err := bf.open(false)
			if err != nil {
				return err
			}
			defer func() { _ = bus.Close() }()
			mst, err := master.New(bus, sf.masterOptions(&bf)...)
			if err != nil {
				return err
			}
			defer func() { _ = mst.Close() }()
			if scanner {
				r.Scanner = se
				err = mst.WriteScannerList(cmd.Context(), node, list, se)
			} else {
				r.Dispatcher = de
				err = mst.WriteDispatcherList(cmd.Context(), node, list, de)
			}
			if err != nil {
				return err
			}
			return a.emit("mpdo "+use, r)
		},
	}
	bf.add(c, true)
	bf.addFD(c, "write over USDO instead of SDO")
	sf.add(c, false)
	sf.addFD(c)
	c.Flags().IntVar(&list, "list", 1, fmt.Sprintf("List number 1..%d", mpdo.MaxLists))
	c.Flags().StringArrayVar(&entries, "entry", nil, "An entry "+format+" (repeatable, comma-separated; none clears the list)")
	return c
}

// mpdoRecord is one MPDO of mpdo watch.
type mpdoRecord struct {
	Timestamp string         `json:"timestamp,omitempty"`
	COBID     string         `json:"cobId"`
	Mode      string         `json:"mode"`
	Node      canopen.NodeID `json:"node"`
	Index     uint16         `json:"index"`
	SubIndex  uint8          `json:"subIndex"`
	Key       string         `json:"key,omitempty"`
	Name      string         `json:"name,omitempty"`
	Data      string         `json:"data"`
	Error     string         `json:"error,omitempty"`
}

type mpdoSummary struct {
	Received  uint64 `json:"received"`
	Malformed uint64 `json:"malformed"`
	Dropped   uint64 `json:"dropped"`
}

func newMPDOWatchCmd(a *app) *cobra.Command {
	var (
		bf       busFlags
		cobs     []string
		edsPath  string
		count    int
		duration time.Duration
	)
	c := &cobra.Command{
		Use:   "watch --cob-id <id>",
		Short: "Decode the MPDOs on COB-IDs (passive)",
		Long: `watch receives the frames of --cob-id (repeatable) and decodes them as MPDOs:
the address mode, the node (DAM: the consumer, 0 for all; SAM: the
producer), the object and its 4 data bytes. --eds names the objects. It
never transmits. A frame shorter than 8 bytes is shown as malformed.`,
		Example: `  canopenctl mpdo watch --cob-id 0x185 --cob-id 0x285 -i vcan0
  canopenctl mpdo watch --cob-id 0x285 --eds node5.eds --count 10 --format json -i can0`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var ids []canbus.Filter
			for _, s := range cobs {
				for _, x := range strings.Split(s, ",") {
					if x = strings.TrimSpace(x); x == "" {
						continue
					}
					cob, err := cli.ParseCOBID(x)
					if err != nil {
						return err
					}
					f := canbus.ExactStandard(cob.ID())
					if cob.Extended() {
						f = canbus.ExactExtended(cob.ID())
					}
					f.Remote = canbus.RemoteDataOnly
					ids = append(ids, f)
				}
			}
			if len(ids) == 0 {
				return errs.Invalid("give --cob-id")
			}
			if count < 0 {
				return errs.Invalid("--count %d: 0 or more", count)
			}
			d, err := loadDesc(edsPath, 1)
			if err != nil {
				return err
			}
			bus, err := bf.open(true)
			if err != nil {
				return err
			}
			defer func() { _ = bus.Close() }()
			sub, err := bus.Subscribe(canbus.WithFilters(canbus.FilterSet{Filters: ids}), canbus.WithQueueSize(65536))
			if err != nil {
				return err
			}
			ctx, cancel := bounded(cmd.Context(), duration)
			defer cancel()
			st := a.stream()
			var sum mpdoSummary
			var shown int
			for count == 0 || shown < count {
				f, err := sub.Receive(ctx)
				if err != nil {
					if ended(ctx, err) {
						break
					}
					return err
				}
				sum.Received++
				r := mpdoRecord{COBID: cli.Hex(uint64(f.ID), 3), Data: hex.EncodeToString(f.Payload())}
				if !f.Timestamp.IsZero() {
					r.Timestamp = stamp(f.Timestamp)
				}
				var text string
				m, err := mpdo.ParseMessage(f.Payload())
				if err != nil {
					sum.Malformed++
					r.Mode, r.Error = "malformed", err.Error()
					text = fmt.Sprintf("(%s) %s MPDO malformed: %v", r.Timestamp, r.COBID, err)
				} else {
					r.Mode, r.Node, r.Index, r.SubIndex, r.Key = "sam", m.Node, m.Index, m.SubIndex, cli.Key(m.Index, m.SubIndex)
					r.Data = hex.EncodeToString(m.Data[:])
					r.Name = d.name(m.Index, m.SubIndex)
					who := fmt.Sprintf("from node %d", m.Node)
					if m.DAM {
						r.Mode, who = "dam", fmt.Sprintf("to node %d", m.Node)
						if m.Node == 0 {
							who = "to all nodes"
						}
					}
					name := ""
					if r.Name != "" {
						name = " " + r.Name
					}
					text = fmt.Sprintf("(%s) %s MPDO %s %s %s%s = %s", r.Timestamp, r.COBID, strings.ToUpper(r.Mode), who, r.Key, name, r.Data)
				}
				if err := st.Record("mpdo", text, r); err != nil {
					return errs.New(errs.KindOutput, errs.CodeOutputFailed, "write output: %w", err)
				}
				shown++
			}
			sum.Dropped = bus.Stats().Dropped + sub.Dropped()
			if a.format == "json" {
				return st.Record("summary", "", sum)
			}
			a.warn("%d MPDOs, %d malformed, %d dropped", sum.Received, sum.Malformed, sum.Dropped)
			return nil
		},
	}
	bf.add(c, false)
	c.Flags().StringArrayVar(&cobs, "cob-id", nil, "COB-ID of MPDOs to decode (hexadecimal; repeatable, comma-separated)")
	c.Flags().StringVar(&edsPath, "eds", "", "EDS or DCF naming the objects")
	c.Flags().IntVarP(&count, "count", "n", 0, "Stop after this many MPDOs (0: no limit)")
	c.Flags().DurationVar(&duration, "duration", 0, "Stop after this time (0: until Ctrl-C)")
	return c
}
