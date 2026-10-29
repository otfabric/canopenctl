// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/otfabric/canopenctl/internal/cli"
	"github.com/otfabric/canopenctl/internal/errs"
	"github.com/otfabric/go-canbus"
	"github.com/otfabric/go-canopen"
	"github.com/otfabric/go-canopen/master"
	"github.com/otfabric/go-canopen/od"
	"github.com/otfabric/go-canopen/pdo"
	"github.com/spf13/cobra"
)

// mapEntry is one PDO mapping entry in a result.
type mapEntry struct {
	Index    uint16 `json:"index"`
	SubIndex uint8  `json:"subIndex"`
	Bits     uint8  `json:"bits"`
	Key      string `json:"key"`
	Name     string `json:"name,omitempty"`
	Dummy    bool   `json:"dummy,omitempty"`
}

// pdoInfo describes the configuration of one PDO.
type pdoInfo struct {
	Kind             string     `json:"kind"`
	PDO              int        `json:"pdo"`
	COBID            string     `json:"cobId"`
	COBIDValue       uint32     `json:"cobIdValue"`
	Valid            bool       `json:"valid"`
	Extended         bool       `json:"extended"`
	RTRAllowed       bool       `json:"rtrAllowed"`
	Active           bool       `json:"active"`
	TransmissionType uint8      `json:"transmissionType"`
	Transmission     string     `json:"transmission"`
	InhibitTime      uint16     `json:"inhibitTime"`
	EventTimer       uint16     `json:"eventTimer"`
	SyncStart        uint8      `json:"syncStart"`
	Bits             int        `json:"bits"`
	Mapping          []mapEntry `json:"mapping"`
}

func pdoKind(transmit bool) string {
	if transmit {
		return "tpdo"
	}
	return "rpdo"
}

func infoOf(transmit bool, n int, c pdo.Config, d *desc) pdoInfo {
	i := pdoInfo{Kind: pdoKind(transmit), PDO: n, COBID: cli.Hex(uint64(c.COBID.ID()), 3), COBIDValue: uint32(c.COBID),
		Valid: c.COBID.Valid(), Extended: c.COBID.Extended(), RTRAllowed: c.COBID&canopen.COBNoRTR == 0, Active: c.Active(),
		TransmissionType: uint8(c.TransmissionType), Transmission: c.TransmissionType.String(), InhibitTime: c.InhibitTime,
		EventTimer: c.EventTimer, SyncStart: c.SyncStart, Bits: c.Mapping.Bits(), Mapping: []mapEntry{}}
	if i.Extended {
		i.COBID = cli.Hex(uint64(c.COBID.ID()), 8)
	}
	for _, e := range c.Mapping {
		me := mapEntry{Index: e.Index, SubIndex: e.SubIndex, Bits: e.Bits, Key: cli.Key(e.Index, e.SubIndex), Name: d.name(e.Index, e.SubIndex), Dummy: e.IsDummy()}
		if me.Dummy {
			me.Name = "dummy " + canopen.DataType(e.Index).String()
		}
		i.Mapping = append(i.Mapping, me)
	}
	return i
}

func (i pdoInfo) text() string {
	var b strings.Builder
	state := "inactive"
	if i.Active {
		state = "active"
	}
	valid := "valid"
	if !i.Valid {
		valid = "invalid"
	}
	fmt.Fprintf(&b, "%s%d  COB-ID %s (%s)  %s  type %d (%s)", strings.ToUpper(i.Kind), i.PDO, i.COBID, valid, state, i.TransmissionType, i.Transmission)
	if i.Kind == "tpdo" {
		fmt.Fprintf(&b, "  inhibit %d×100µs", i.InhibitTime)
	}
	fmt.Fprintf(&b, "  event timer %d ms", i.EventTimer)
	if i.SyncStart != 0 {
		fmt.Fprintf(&b, "  sync start %d", i.SyncStart)
	}
	for _, m := range i.Mapping {
		fmt.Fprintf(&b, "\n    %s/%d", m.Key, m.Bits)
		if m.Name != "" {
			fmt.Fprintf(&b, "  %s", m.Name)
		}
	}
	return b.String()
}

type pdoShowResult struct {
	Node canopen.NodeID `json:"node"`
	PDOs []pdoInfo      `json:"pdos"`
}

func (r pdoShowResult) Text() string {
	if len(r.PDOs) == 0 {
		return fmt.Sprintf("node %d: no PDOs", r.Node)
	}
	var parts []string
	for _, p := range r.PDOs {
		parts = append(parts, p.text())
	}
	return strings.Join(parts, "\n")
}

func (r pdoShowResult) Headers() []string {
	return []string{"PDO", "COB-ID", "VALID", "TYPE", "INHIBIT", "EVENT-TIMER", "MAPPING"}
}

func (r pdoShowResult) Rows() [][]string {
	var rows [][]string
	for _, p := range r.PDOs {
		var m []string
		for _, e := range p.Mapping {
			m = append(m, fmt.Sprintf("%s/%d", e.Key, e.Bits))
		}
		rows = append(rows, []string{strings.ToUpper(p.Kind) + strconv.Itoa(p.PDO), p.COBID, strconv.FormatBool(p.Valid),
			strconv.Itoa(int(p.TransmissionType)), strconv.Itoa(int(p.InhibitTime)), strconv.Itoa(int(p.EventTimer)), strings.Join(m, ",")})
	}
	return rows
}

func newPDOCmd(a *app) *cobra.Command {
	c := &cobra.Command{
		Use:   "pdo",
		Short: "Show, map and watch a node's process data objects",
	}
	c.AddCommand(newPDOShowCmd(a), newPDOMapCmd(a), newPDOWatchCmd(a))
	return c
}

// isAbsent reports an SDO abort saying an object does not exist.
func isAbsent(err error) bool {
	var ae *canopen.AbortError
	return errors.As(err, &ae) && ae.Remote && (ae.Code == canopen.AbortNoObject || ae.Code == canopen.AbortNoSubIndex)
}

func newPDOShowCmd(a *app) *cobra.Command {
	var (
		bf         busFlags
		sf         sdoFlags
		tpdo, rpdo int
		edsPath    string
	)
	c := &cobra.Command{
		Use:   "show <node>",
		Short: "Read a node's PDO communication and mapping parameters (SDO reads)",
		Long: `show reads the communication (0x1400.., 0x1800..) and mapping (0x1600..,
0x1A00..) parameters of the node's PDOs by SDO: TPDO 1..4 and RPDO 1..4, or
the one of --tpdo or --rpdo. PDOs the node does not have are left out (with
--tpdo/--rpdo they are an error). --eds names the mapped objects. It only
sends SDO read requests.`,
		Example: `  canopenctl pdo show 5 -i can0
  canopenctl pdo show 5 --tpdo 1 --eds io401.eds -i vcan0`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			node, err := cli.ParseNode(args[0], false)
			if err != nil {
				return err
			}
			if err := sf.validate(); err != nil {
				return err
			}
			if tpdo != 0 && rpdo != 0 {
				return errs.Invalid("give --tpdo or --rpdo, not both")
			}
			type sel struct {
				transmit bool
				n        int
			}
			var list []sel
			switch {
			case tpdo != 0:
				list = []sel{{true, tpdo}}
			case rpdo != 0:
				list = []sel{{false, rpdo}}
			default:
				for _, t := range []bool{true, false} {
					for n := 1; n <= 4; n++ {
						list = append(list, sel{t, n})
					}
				}
			}
			for _, s := range list {
				if s.n < 1 || s.n > od.MaxPDOs {
					return errs.Invalid("PDO number %d: 1..512", s.n)
				}
			}
			d, err := loadDesc(edsPath, node)
			if err != nil {
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
			cl, err := newRemote(bus, node, &sf, &bf)
			if err != nil {
				return err
			}
			defer func() { _ = cl.Close() }()
			r := pdoShowResult{Node: node, PDOs: []pdoInfo{}}
			for _, s := range list {
				cfg, err := pdo.ReadConfig(cmd.Context(), cl, s.transmit, s.n)
				if err != nil {
					if len(list) > 1 && isAbsent(err) {
						continue
					}
					return err
				}
				r.PDOs = append(r.PDOs, infoOf(s.transmit, s.n, cfg, d))
			}
			return a.emit("pdo show", r)
		},
	}
	bf.add(c, false)
	bf.addFD(c, "read over USDO instead of SDO")
	sf.add(c, false)
	sf.addFD(c)
	c.Flags().IntVar(&tpdo, "tpdo", 0, "Only this TPDO (1..512)")
	c.Flags().IntVar(&rpdo, "rpdo", 0, "Only this RPDO (1..512)")
	c.Flags().StringVar(&edsPath, "eds", "", "EDS or DCF of the node: names of the mapped objects")
	return c
}

type pdoMapResult struct {
	Node canopen.NodeID `json:"node"`
	pdoInfo
}

func (r pdoMapResult) Text() string {
	return fmt.Sprintf("node %d configured:\n%s", r.Node, r.text())
}

func newPDOMapCmd(a *app) *cobra.Command {
	var (
		bf                         busFlags
		sf                         sdoFlags
		tpdo, rpdo                 int
		mapping, cob, edsPath      string
		trans                      int
		inhibit, eventTimer, start int
		disable, noRTR             bool
	)
	c := &cobra.Command{
		Use:   "map <node> (--tpdo n | --rpdo n) --map IIII:SS:BITS,...",
		Short: "Configure a PDO's mapping and communication parameters (needs --yes off virtual interfaces)",
		Long: `map writes a PDO's configuration by SDO in the order CiA 301 requires:
invalidate the COB-ID, clear the mapping, write the entries, enable the
mapping, the transmission type, inhibit time, event timer and SYNC start
value, and make the COB-ID valid. Parameters not given keep the node's
values. Entries are IIII:SS:BITS (index and
sub-index hexadecimal, length in bits), comma-separated; dummy entries
(0002..0007:00) skip bits of an RPDO. The COB-ID is kept from the node unless
--cob-id is given; --disable leaves the PDO invalid. --eds checks the mapping
against the node's description before anything is sent. Most devices accept
mapping changes only in pre-operational.`,
		Example: `  canopenctl pdo map 5 --tpdo 1 --map 0x6041:00:16,0x6064:00:32 --type 255 --event-timer 100 -i vcan0
  canopenctl pdo map 5 --rpdo 1 --map 6200:01:8,6200:02:8 --type 1 -i can0 --yes
  canopenctl pdo map 5 --tpdo 2 --disable -i can0 --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			node, err := cli.ParseNode(args[0], false)
			if err != nil {
				return err
			}
			if err := sf.validate(); err != nil {
				return err
			}
			if (tpdo == 0) == (rpdo == 0) {
				return errs.Invalid("give --tpdo or --rpdo")
			}
			transmit, n := tpdo != 0, max(tpdo, rpdo)
			if n < 1 || n > od.MaxPDOs {
				return errs.Invalid("PDO number %d: 1..512", n)
			}
			m, err := cli.ParseMapping(mapping)
			if err != nil {
				return err
			}
			if len(m) == 0 && !disable {
				return errs.Invalid("give --map (or --disable)")
			}
			if trans < 0 || trans > 255 || !pdo.TransmissionType(trans).ValidFor(transmit) {
				return errs.Invalid("--type %d: 0..240, 254, 255 (and 252, 253 for a TPDO)", trans)
			}
			if inhibit < 0 || inhibit > 0xFFFF || eventTimer < 0 || eventTimer > 0xFFFF {
				return errs.Invalid("--inhibit and --event-timer: 0..65535")
			}
			if start < 0 || start > 240 {
				return errs.Invalid("--sync-start %d: 0..240", start)
			}
			if !transmit && (inhibit != 0 || start != 0) {
				return errs.Invalid("--inhibit and --sync-start apply to TPDOs only")
			}
			d, err := loadDesc(edsPath, node)
			if err != nil {
				return err
			}
			if d != nil && len(m) > 0 {
				if err := m.Validate(d.dict, transmit); err != nil {
					return errs.Invalid("mapping does not fit %s: %v", edsPath, err)
				}
			}
			var cobID canopen.COBID
			if cob != "" {
				if cobID, err = cli.ParseCOBID(cob); err != nil {
					return err
				}
				if !cobID.Extended() && pdo.Restricted(cobID.ID()) {
					return errs.Invalid("--cob-id %s is reserved by CiA 301 (NMT, SYNC, EMCY, SDO, heartbeat, LSS)", cob)
				}
			}
			if err := bf.checkFD(); err != nil {
				return err
			}
			if err := bf.check("pdo map"); err != nil {
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
			cur, err := mst.ReadPDO(cmd.Context(), node, transmit, n)
			if err != nil {
				return err
			}
			cfg := cur
			if cob != "" {
				cfg.COBID = cobID | cur.COBID&canopen.COBNoRTR
			}
			cfg.COBID &^= canopen.COBInvalid
			if noRTR {
				cfg.COBID |= canopen.COBNoRTR
			}
			if disable {
				cfg.COBID |= canopen.COBInvalid
			}
			if len(m) > 0 {
				cfg.Mapping = m
			}
			fl := cmd.Flags()
			if fl.Changed("type") {
				cfg.TransmissionType = pdo.TransmissionType(trans)
			}
			if fl.Changed("inhibit") {
				cfg.InhibitTime = uint16(inhibit)
			}
			if fl.Changed("event-timer") {
				cfg.EventTimer = uint16(eventTimer)
			}
			if fl.Changed("sync-start") {
				cfg.SyncStart = uint8(start)
			}
			if err := mst.ConfigurePDO(cmd.Context(), node, transmit, n, cfg); err != nil {
				return err
			}
			back, err := mst.ReadPDO(cmd.Context(), node, transmit, n)
			if err != nil {
				return fmt.Errorf("read back: %w", err)
			}
			return a.emit("pdo map", pdoMapResult{Node: node, pdoInfo: infoOf(transmit, n, back, d)})
		},
	}
	bf.add(c, true)
	bf.addFD(c, "write over USDO instead of SDO")
	sf.add(c, false)
	sf.addFD(c)
	c.Flags().IntVar(&tpdo, "tpdo", 0, "Configure this TPDO (1..512)")
	c.Flags().IntVar(&rpdo, "rpdo", 0, "Configure this RPDO (1..512)")
	c.Flags().StringVar(&mapping, "map", "", "Mapping entries IIII:SS:BITS, comma-separated")
	c.Flags().IntVar(&trans, "type", 255, "Transmission type: 0 acyclic sync, 1..240 every nth SYNC, 252/253 RTR (TPDO), 254/255 event-driven (default: keep the node's)")
	c.Flags().IntVar(&inhibit, "inhibit", 0, "Inhibit time of a TPDO in 100 µs (default: keep the node's)")
	c.Flags().IntVar(&eventTimer, "event-timer", 0, "Event timer (TPDO) or deadline (RPDO) in ms (default: keep the node's)")
	c.Flags().IntVar(&start, "sync-start", 0, "SYNC start value of a TPDO, 0..240 (default: keep the node's)")
	c.Flags().StringVar(&cob, "cob-id", "", "CAN identifier of the PDO (hexadecimal; default: keep the node's)")
	c.Flags().BoolVar(&noRTR, "no-rtr", false, "Set COB-ID bit 30: no remote transmission requests")
	c.Flags().BoolVar(&disable, "disable", false, "Leave the PDO invalid (COB-ID bit 31)")
	c.Flags().StringVar(&edsPath, "eds", "", "EDS or DCF of the node: check the mapping before sending")
	return c
}

// guessDictionary describes the objects of a mapping without an EDS: an
// unsigned integer of each entry's length (BOOLEAN for 1 bit).
func guessDictionary(m pdo.Mapping) (*od.Dictionary, error) {
	d := od.New(0)
	subs := map[uint16][]*od.Variable{}
	var order []uint16
	for _, e := range m {
		if e.IsDummy() {
			continue
		}
		dt := map[uint8]canopen.DataType{1: canopen.Boolean, 8: canopen.Unsigned8, 16: canopen.Unsigned16, 24: canopen.Unsigned24,
			32: canopen.Unsigned32, 40: canopen.Unsigned40, 48: canopen.Unsigned48, 56: canopen.Unsigned56, 64: canopen.Unsigned64}[e.Bits]
		if dt == 0 {
			return nil, errs.Invalid("mapping entry %s: %d bits fit no data type; give --eds", e, e.Bits)
		}
		if _, ok := subs[e.Index]; !ok {
			order = append(order, e.Index)
		}
		dup := false
		for _, v := range subs[e.Index] {
			dup = dup || v.SubIndex == e.SubIndex
		}
		if !dup {
			subs[e.Index] = append(subs[e.Index], od.Sub(e.SubIndex, fmt.Sprintf("0x%04X:%02X", e.Index, e.SubIndex), dt, od.AccessRW, nil))
		}
	}
	for _, idx := range order {
		vs := subs[idx]
		var o *od.Object
		if len(vs) == 1 && vs[0].SubIndex == 0 {
			o = od.NewVar(idx, fmt.Sprintf("0x%04X", idx), vs[0].DataType, od.AccessRW, nil)
		} else {
			sortVars(vs)
			if vs[0].SubIndex == 0 {
				vs = vs[1:] // sub 0 of a record is its size
			}
			o = od.NewRecord(idx, fmt.Sprintf("0x%04X", idx), vs...)
		}
		if err := d.Add(o); err != nil {
			return nil, errs.Invalid("mapping: %v; give --eds", err)
		}
	}
	return d, nil
}

func sortVars(vs []*od.Variable) {
	for i := 1; i < len(vs); i++ {
		for j := i; j > 0 && vs[j].SubIndex < vs[j-1].SubIndex; j-- {
			vs[j], vs[j-1] = vs[j-1], vs[j]
		}
	}
}

// pdoValue is one decoded value of a PDO record.
type pdoValue struct {
	Index    uint16 `json:"index"`
	SubIndex uint8  `json:"subIndex"`
	Key      string `json:"key"`
	Name     string `json:"name,omitempty"`
	Type     string `json:"type"`
	Raw      string `json:"raw"`
	Value    any    `json:"value"`
	text     string
}

type pdoRecord struct {
	Timestamp string         `json:"timestamp,omitempty"`
	Node      canopen.NodeID `json:"node"`
	Kind      string         `json:"kind"`
	PDO       int            `json:"pdo"`
	COBID     string         `json:"cobId"`
	Data      string         `json:"data"`
	Values    []pdoValue     `json:"values"`
}

type pdoWatchSummary struct {
	Received uint64 `json:"received"`
	Errors   uint64 `json:"errors"`
}

func newPDOWatchCmd(a *app) *cobra.Command {
	var (
		bf               busFlags
		sf               sdoFlags
		tpdo             int
		edsPath, cob, mp string
		count            int
		duration         time.Duration
	)
	c := &cobra.Command{
		Use:   "watch <node> --tpdo n",
		Short: "Print a node's TPDO with its decoded values",
		Long: `watch reads the TPDO's COB-ID and mapping from the node by SDO (or takes them
from --cob-id and --map, sending nothing), then receives the PDO and prints
each one with its values. --eds gives the names and data types of the mapped
objects; without it every value is an unsigned integer of its length. The
node must be operational to transmit.`,
		Example: `  canopenctl pdo watch 5 --tpdo 1 --eds io401.eds -i can0
  canopenctl pdo watch 5 --tpdo 1 --cob-id 185 --map 6000:01:8,6000:02:8 -i can0 --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			node, err := cli.ParseNode(args[0], false)
			if err != nil {
				return err
			}
			if err := sf.validate(); err != nil {
				return err
			}
			if tpdo < 1 || tpdo > od.MaxPDOs {
				return errs.Invalid("--tpdo %d: 1..512", tpdo)
			}
			if (cob == "") != (mp == "") {
				return errs.Invalid("--cob-id and --map go together")
			}
			d, err := loadDesc(edsPath, node)
			if err != nil {
				return err
			}
			passive := cob != ""
			bus, err := openBus(bf.iface, openOpts{passive: passive})
			if err != nil {
				return err
			}
			defer func() { _ = bus.Close() }()
			var cfg pdo.Config
			if passive {
				if cfg.COBID, err = cli.ParseCOBID(cob); err != nil {
					return err
				}
				if cfg.Mapping, err = cli.ParseMapping(mp); err != nil {
					return err
				}
			} else {
				cl, err := sdoClient(bus, node, &sf)
				if err != nil {
					return err
				}
				cfg, err = pdo.ReadConfig(cmd.Context(), sdoAdapter{cl}, true, tpdo)
				_ = cl.Close()
				if err != nil {
					return err
				}
				if !cfg.Active() {
					return errs.New(errs.KindDevice, errs.CodePDOInactive, "TPDO%d of node %d is not active (COB-ID %s, %d mapped objects)", tpdo, node, cfg.COBID, len(cfg.Mapping))
				}
			}
			dict := (*od.Dictionary)(nil)
			if d != nil {
				dict = d.dict
			} else if dict, err = guessDictionary(cfg.Mapping); err != nil {
				return err
			}
			dec, err := pdo.NewDecoder(cfg.Mapping, dict)
			if err != nil {
				return errs.Invalid("mapping %s: %v", cfg.Mapping, err)
			}
			type got struct {
				f      canbus.Frame
				values []pdo.Value
			}
			ch := make(chan got, 1024)
			var mu sync.Mutex
			closed := false
			lst := pdo.NewListener(bus, cfg.COBID&^canopen.COBInvalid, dec, func(f *canbus.Frame, values []pdo.Value) {
				mu.Lock()
				defer mu.Unlock()
				if closed {
					return
				}
				select {
				case ch <- got{*f, values}:
				default:
				}
			})
			if err := lst.Start(); err != nil {
				return err
			}
			defer func() {
				mu.Lock()
				closed = true
				mu.Unlock()
				_ = lst.Close()
			}()
			ctx, cancel := bounded(cmd.Context(), duration)
			defer cancel()
			st := a.stream()
			shown := 0
		loop:
			for count == 0 || shown < count {
				select {
				case <-ctx.Done():
					break loop
				case g := <-ch:
					r := pdoRecord{Node: node, Kind: "tpdo", PDO: tpdo, COBID: cli.Hex(uint64(cfg.COBID.ID()), 3), Data: hex.EncodeToString(g.f.Payload()), Values: []pdoValue{}}
					if !g.f.Timestamp.IsZero() {
						r.Timestamp = stamp(g.f.Timestamp)
					}
					var parts []string
					for _, v := range g.values {
						pv := pdoValue{Index: v.Index, SubIndex: v.SubIndex, Key: cli.Key(v.Index, v.SubIndex), Name: d.name(v.Index, v.SubIndex),
							Type: v.Type.String(), Raw: hex.EncodeToString(v.Raw)}
						pv.Value, _ = cli.JSONValue(v.Type, v.Raw)
						pv.text = cli.ValueText(v.Type, v.Raw)
						label := pv.Name
						if label == "" {
							label = pv.Key
						}
						parts = append(parts, label+"="+pv.text)
						r.Values = append(r.Values, pv)
					}
					text := fmt.Sprintf("(%s) node %d TPDO%d %s#%s  %s", r.Timestamp, node, tpdo, r.COBID[2:], r.Data, strings.Join(parts, "  "))
					if err := st.Record("pdo", text, r); err != nil {
						return errs.New(errs.KindOutput, errs.CodeOutputFailed, "write output: %w", err)
					}
					shown++
				}
			}
			s := lst.Stats()
			sum := pdoWatchSummary{Received: s.Received, Errors: s.Errors}
			if a.format == "json" {
				return st.Record("summary", "", sum)
			}
			a.warn("%d PDOs received, %d too short for the mapping", sum.Received, sum.Errors)
			return nil
		},
	}
	bf.add(c, false)
	sf.add(c, false)
	c.Flags().IntVar(&tpdo, "tpdo", 0, "TPDO number (1..512, required)")
	c.Flags().StringVar(&edsPath, "eds", "", "EDS or DCF of the node: names and data types of the mapped objects")
	c.Flags().StringVar(&cob, "cob-id", "", "COB-ID of the PDO (hexadecimal; with --map: read nothing from the node)")
	c.Flags().StringVar(&mp, "map", "", "Mapping IIII:SS:BITS,... (with --cob-id)")
	c.Flags().IntVarP(&count, "count", "n", 0, "Stop after this many PDOs (0: no limit)")
	c.Flags().DurationVar(&duration, "duration", 0, "Stop after this time (0: until Ctrl-C)")
	_ = c.MarkFlagRequired("tpdo")
	return c
}
