// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/otfabric/canopenctl/internal/cli"
	"github.com/otfabric/canopenctl/internal/errs"
	"github.com/otfabric/go-canopen"
	"github.com/otfabric/go-canopen/eds"
	"github.com/otfabric/go-canopen/od"
	"github.com/otfabric/go-canopen/sdo"
	"github.com/spf13/cobra"
)

// desc is an EDS or DCF loaded to describe a node: names, types and
// access rights of its objects.
type desc struct {
	path string
	file *eds.File
	dict *od.Dictionary
}

// loadDesc loads an EDS for a node; "" loads nothing (nil, nil).
func loadDesc(path string, node canopen.NodeID) (*desc, error) {
	if path == "" {
		return nil, nil
	}
	f, err := cli.LoadEDS(path)
	if err != nil {
		return nil, err
	}
	d, _, err := cli.Dictionary(f, path, node, true)
	if err != nil {
		return nil, err
	}
	return &desc{path: path, file: f, dict: d}, nil
}

// variable returns the description of a sub-object, or nil.
func (d *desc) variable(index uint16, sub uint8) *od.Variable {
	if d == nil {
		return nil
	}
	v, err := d.dict.Variable(index, sub)
	if err != nil {
		return nil
	}
	return v
}

// name returns "Object name" or "Object name / Sub name" of a
// sub-object; "" when unknown.
func (d *desc) name(index uint16, sub uint8) string {
	if d == nil {
		return ""
	}
	o := d.dict.Object(index)
	if o == nil {
		return ""
	}
	v := o.Sub(sub)
	if o.Type == od.ObjectVar || o.Type == od.ObjectDomain || v == nil || v.Name == "" || v.Name == o.Name {
		return o.Name
	}
	return o.Name + " / " + v.Name
}

// sdoValue is the result of sdo read and sdo write.
type sdoValue struct {
	Node       canopen.NodeID `json:"node"`
	Index      uint16         `json:"index"`
	SubIndex   uint8          `json:"subIndex"`
	Key        string         `json:"key"`
	Name       string         `json:"name,omitempty"`
	Type       string         `json:"type"`
	TypeSource string         `json:"typeSource"`
	Size       int            `json:"size"`
	Data       string         `json:"data"`
	Value      any            `json:"value"`
	Block      bool           `json:"block"`
	Verified   *bool          `json:"verified,omitempty"`
	File       string         `json:"file,omitempty"`
	write      bool
	text       string
}

func (r sdoValue) Text() string {
	name := ""
	if r.Name != "" {
		name = " " + r.Name
	}
	if r.write {
		s := fmt.Sprintf("node %d %s%s (%s) <- %s: written (%d bytes)", r.Node, r.Key, name, r.Type, r.text, r.Size)
		if r.Verified != nil {
			s += ", verified"
		}
		return s
	}
	if r.File != "" {
		return fmt.Sprintf("node %d %s%s: %d bytes written to %s", r.Node, r.Key, name, r.Size, r.File)
	}
	return fmt.Sprintf("node %d %s%s (%s) = %s", r.Node, r.Key, name, r.Type, r.text)
}

func (r sdoValue) Headers() []string { return []string{"NODE", "KEY", "NAME", "TYPE", "SIZE", "VALUE"} }
func (r sdoValue) Rows() [][]string {
	return [][]string{{strconv.Itoa(int(r.Node)), r.Key, r.Name, r.Type, strconv.Itoa(r.Size), r.text}}
}

// sdoTarget is the parsed <node> <index> <sub> of an sdo command.
type sdoTarget struct {
	node  canopen.NodeID
	index uint16
	sub   uint8
}

func parseTarget(args []string) (sdoTarget, error) {
	var t sdoTarget
	var err error
	if t.node, err = cli.ParseNode(args[0], false); err != nil {
		return t, err
	}
	if t.index, err = cli.ParseIndex(args[1]); err != nil {
		return t, err
	}
	t.sub, err = cli.ParseSub(args[2])
	return t, err
}

// typeOf chooses the data type of a value: --type, else the EDS, else none.
func typeOf(flag string, d *desc, index uint16, sub uint8) (canopen.DataType, string, error) {
	if flag != "" {
		t, err := cli.ParseType(flag)
		return t, "flag", err
	}
	if v := d.variable(index, sub); v != nil {
		return v.DataType, "eds", nil
	}
	return 0, "", nil
}

// sdoClient opens a client for a node on an open bus.
func sdoClient(bus canopen.Bus, node canopen.NodeID, sf *sdoFlags) (*sdo.Client, error) {
	return sdo.NewClient(bus, node, sf.options()...)
}

func newSDOCmd(a *app) *cobra.Command {
	c := &cobra.Command{
		Use:   "sdo",
		Short: "Read and write a node's object dictionary (SDO client)",
		Long: `The sdo commands are an SDO client (CiA 301): expedited, segmented and block
transfer (--block). A device that refuses a transfer answers with an SDO abort
code: it is printed with its meaning and ends the command with exit code 11
(JSON: the error's "abort" object). Indices are hexadecimal (1018, 0x1018);
sub-indices decimal or 0x hexadecimal.`,
	}
	c.AddCommand(newSDOReadCmd(a), newSDOWriteCmd(a), newSDODumpCmd(a))
	return c
}

func newSDOReadCmd(a *app) *cobra.Command {
	var (
		bf      busFlags
		sf      sdoFlags
		typ     string
		edsPath string
		outPath string
	)
	c := &cobra.Command{
		Use:   "read <node> <index> <sub>",
		Short: "Read (upload) a sub-object of a node",
		Long: `read uploads a sub-object and prints its value. The data type comes from
--type, else from --eds, else it is guessed from the length (an unsigned
integer for 1..8 bytes, text for printable data, bytes otherwise; JSON says
"typeSource": "guess"). --output writes the raw data to a file (DOMAIN
objects). read only sends SDO read requests: it needs no --yes.`,
		Example: `  canopenctl sdo read 5 0x1018 1 -i can0
  canopenctl sdo read 5 1008 0 --type string -i vcan0
  canopenctl sdo read 5 0x2010 0 --block -o firmware.bin -i can0`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := parseTarget(args)
			if err != nil {
				return err
			}
			if err := sf.validate(); err != nil {
				return err
			}
			d, err := loadDesc(edsPath, t.node)
			if err != nil {
				return err
			}
			dt, src, err := typeOf(typ, d, t.index, t.sub)
			if err != nil {
				return err
			}
			bus, err := openBus(bf.iface, openOpts{})
			if err != nil {
				return err
			}
			defer func() { _ = bus.Close() }()
			cl, err := sdoClient(bus, t.node, &sf)
			if err != nil {
				return err
			}
			defer func() { _ = cl.Close() }()
			data, err := cl.Upload(cmd.Context(), t.index, t.sub)
			if err != nil {
				return err
			}
			r := sdoValue{Node: t.node, Index: t.index, SubIndex: t.sub, Key: cli.Key(t.index, t.sub), Name: d.name(t.index, t.sub),
				Size: len(data), Data: fmt.Sprintf("%x", data), Block: cl.Stats().BlockTransfers > 0}
			if outPath != "" {
				if err := os.WriteFile(outPath, data, 0o644); err != nil {
					return errs.New(errs.KindOutput, errs.CodeOutputFailed, "write %s: %w", outPath, err)
				}
				r.File = outPath
			}
			if src == "" {
				dt, src = cli.GuessType(data), "guess"
			}
			v, ok := cli.JSONValue(dt, data)
			if !ok {
				if src == "flag" {
					return errs.New(errs.KindProtocol, errs.CodeDataType, "%s of node %d: %d bytes are not a %s value (data %x)", r.Key, t.node, len(data), dt, data)
				}
				a.warn("%s: %d bytes are not a %s value as %s says; shown as bytes", r.Key, len(data), dt, edsPath)
				dt, src = canopen.OctetString, "guess"
				v, _ = cli.JSONValue(dt, data)
			}
			r.Type, r.TypeSource, r.Value, r.text = dt.String(), src, v, cli.ValueText(dt, data)
			return a.emit("sdo read", r)
		},
	}
	bf.add(c, false)
	sf.add(c, true)
	c.Flags().StringVarP(&typ, "type", "t", "", "Data type: UNSIGNED32, INTEGER16, REAL32, VISIBLE_STRING, OCTET_STRING, DOMAIN... or u32, i16, f32, string, octets")
	c.Flags().StringVar(&edsPath, "eds", "", "EDS or DCF of the node: names and data types")
	c.Flags().StringVarP(&outPath, "output", "o", "", "Also write the raw data to this file")
	return c
}

func newSDOWriteCmd(a *app) *cobra.Command {
	var (
		bf      busFlags
		sf      sdoFlags
		typ     string
		edsPath string
		inPath  string
		verify  bool
	)
	c := &cobra.Command{
		Use:   "write <node> <index> <sub> [value]",
		Short: "Write (download) a sub-object of a node (needs --yes off virtual interfaces)",
		Long: `write downloads a value to a sub-object. The data type comes from --type or
--eds (one is required). Integers are decimal, 0x hexadecimal or octal with a
leading 0 (negative ones with a sign; $NODEID+0x180 is evaluated for the
node); floats decimal; BOOLEAN 0/1/true/false; strings as text; OCTET_STRING
and DOMAIN as hexadecimal digit pairs. --file downloads a file's content
instead of a value (DOMAIN; use --block for large files). --verify reads the
value back and exits 1 when it differs.`,
		Example: `  canopenctl sdo write 5 0x1017 0 1000 --type u16 -i vcan0
  canopenctl sdo write 5 0x6200 1 0xFF --eds io401.eds -i can0 --yes
  canopenctl sdo write 5 0x1F50 1 --file firmware.bin --block -i can0 --yes`,
		Args: cobra.RangeArgs(3, 4),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := parseTarget(args)
			if err != nil {
				return err
			}
			if err := sf.validate(); err != nil {
				return err
			}
			if (len(args) == 4) == (inPath != "") {
				return errs.Invalid("give the value or --file, not both or neither")
			}
			d, err := loadDesc(edsPath, t.node)
			if err != nil {
				return err
			}
			dt, src, err := typeOf(typ, d, t.index, t.sub)
			if err != nil {
				return err
			}
			if src == "" && inPath == "" {
				if d != nil {
					return errs.Invalid("%s is not in %s: give --type", cli.Key(t.index, t.sub), edsPath)
				}
				return errs.Invalid("the data type is unknown: give --type or --eds")
			}
			var data []byte
			text := ""
			if inPath != "" {
				if data, err = os.ReadFile(inPath); err != nil {
					return errs.New(errs.KindInvalidInput, errs.CodeFileOpen, "cannot read %s: %w", inPath, err)
				}
				if src == "" {
					dt, src = canopen.Domain, "file"
				}
				if dt.Size() > 0 && len(data) != dt.Size() {
					return errs.New(errs.KindInvalidInput, errs.CodeInvalidValue, "%s holds %d bytes, a %s has %d", inPath, len(data), dt, dt.Size())
				}
				text = fmt.Sprintf("%d bytes of %s", len(data), inPath)
			} else {
				if data, err = cli.ParseValue(dt, args[3], t.node); err != nil {
					return err
				}
				text = cli.ValueText(dt, data)
			}
			if err := bf.check("sdo write"); err != nil {
				return err
			}
			bus, err := openBus(bf.iface, openOpts{})
			if err != nil {
				return err
			}
			defer func() { _ = bus.Close() }()
			cl, err := sdoClient(bus, t.node, &sf)
			if err != nil {
				return err
			}
			defer func() { _ = cl.Close() }()
			if err := cl.Download(cmd.Context(), t.index, t.sub, data); err != nil {
				return err
			}
			v, _ := cli.JSONValue(dt, data)
			r := sdoValue{Node: t.node, Index: t.index, SubIndex: t.sub, Key: cli.Key(t.index, t.sub), Name: d.name(t.index, t.sub),
				Type: dt.String(), TypeSource: src, Size: len(data), Data: fmt.Sprintf("%x", data), Value: v,
				Block: cl.Stats().BlockTransfers > 0, File: inPath, write: true, text: text}
			if verify {
				back, err := cl.Upload(cmd.Context(), t.index, t.sub)
				if err != nil {
					return fmt.Errorf("verify: %w", err)
				}
				same := bytes.Equal(back, data)
				r.Verified = &same
				if !same {
					if err := a.emit("sdo write", r); err != nil {
						return err
					}
					a.warn("verify: %s reads back as %x, not %x", r.Key, back, data)
					return errs.ErrFindings
				}
			}
			return a.emit("sdo write", r)
		},
	}
	bf.add(c, true)
	sf.add(c, true)
	c.Flags().StringVarP(&typ, "type", "t", "", "Data type: UNSIGNED32, INTEGER16, REAL32, VISIBLE_STRING, OCTET_STRING, DOMAIN... or u32, i16, f32, string, octets")
	c.Flags().StringVar(&edsPath, "eds", "", "EDS or DCF of the node: data types and names")
	c.Flags().StringVar(&inPath, "file", "", "Download this file's content instead of a value")
	c.Flags().BoolVar(&verify, "verify", false, "Read the value back and compare (exit 1 when it differs)")
	return c
}

// dumpEntry is one sub-object of sdo dump.
type dumpEntry struct {
	Index    uint16 `json:"index"`
	SubIndex uint8  `json:"subIndex"`
	Key      string `json:"key"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Access   string `json:"access"`
	Data     string `json:"data,omitempty"`
	Value    any    `json:"value,omitempty"`
	Error    string `json:"error,omitempty"`
	Abort    string `json:"abortCode,omitempty"`
	text     string
}

type dumpResult struct {
	Node    canopen.NodeID `json:"node"`
	EDS     string         `json:"eds"`
	Objects []dumpEntry    `json:"objects"`
	Read    int            `json:"read"`
	Failed  int            `json:"failed"`
}

func (r dumpResult) Text() string {
	var b strings.Builder
	for _, e := range r.Objects {
		v := e.text
		if e.Error != "" {
			v = "error: " + e.Error
		}
		fmt.Fprintf(&b, "%s  %-40s %-15s %s\n", e.Key, e.Name, e.Type, v)
	}
	fmt.Fprintf(&b, "node %d: %d read, %d failed", r.Node, r.Read, r.Failed)
	return b.String()
}

func (r dumpResult) Headers() []string { return []string{"KEY", "NAME", "TYPE", "ACCESS", "VALUE"} }
func (r dumpResult) Rows() [][]string {
	var rows [][]string
	for _, e := range r.Objects {
		v := e.text
		if e.Error != "" {
			v = "error: " + e.Error
		}
		rows = append(rows, []string{e.Key, e.Name, e.Type, e.Access, v})
	}
	return rows
}

// indexRange parses --range lo-hi.
func indexRange(s string) (uint16, uint16, error) {
	if s == "" {
		return 0, 0xFFFF, nil
	}
	lo, hi, ok := strings.Cut(s, "-")
	if !ok {
		hi = lo
	}
	a, err := cli.ParseIndex(lo)
	if err != nil {
		return 0, 0, errs.Invalid("invalid --range %q (index or lo-hi, hexadecimal)", s)
	}
	b, err := cli.ParseIndex(hi)
	if err != nil || b < a {
		return 0, 0, errs.Invalid("invalid --range %q (index or lo-hi, hexadecimal)", s)
	}
	return a, b, nil
}

func newSDODumpCmd(a *app) *cobra.Command {
	var (
		bf      busFlags
		sf      sdoFlags
		edsPath string
		rng     string
	)
	c := &cobra.Command{
		Use:   "dump <node> --eds <file>",
		Short: "Read every readable object an EDS lists from a node",
		Long: `dump reads every readable sub-object the EDS describes (or those of --range)
from the node and prints them with their names and decoded values. Objects
the node refuses are listed with the abort code; any failure makes the exit
code 1. When the node does not answer at all, dump stops with a timeout (exit
5).`,
		Example: `  canopenctl sdo dump 5 --eds io401.eds -i can0
  canopenctl sdo dump 5 --eds drive.eds --range 6000-6FFF --format table -i vcan0`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			node, err := cli.ParseNode(args[0], false)
			if err != nil {
				return err
			}
			if err := sf.validate(); err != nil {
				return err
			}
			lo, hi, err := indexRange(rng)
			if err != nil {
				return err
			}
			d, err := loadDesc(edsPath, node)
			if err != nil {
				return err
			}
			bus, err := openBus(bf.iface, openOpts{})
			if err != nil {
				return err
			}
			defer func() { _ = bus.Close() }()
			cl, err := sdoClient(bus, node, &sf)
			if err != nil {
				return err
			}
			defer func() { _ = cl.Close() }()
			res := dumpResult{Node: node, EDS: edsPath, Objects: []dumpEntry{}}
			timeouts := 0
			for _, o := range d.dict.Objects() {
				if o.Index < lo || o.Index > hi {
					continue
				}
				for _, v := range o.Subs {
					if !v.Access.Readable() {
						continue
					}
					e := dumpEntry{Index: o.Index, SubIndex: v.SubIndex, Key: cli.Key(o.Index, v.SubIndex), Name: d.name(o.Index, v.SubIndex),
						Type: v.DataType.String(), Access: v.Access.String()}
					data, err := cl.Upload(cmd.Context(), o.Index, v.SubIndex)
					if err != nil {
						if cmd.Context().Err() != nil {
							return err
						}
						if errors.Is(err, canopen.ErrTimeout) {
							timeouts++
							if res.Read == 0 || timeouts >= 3 {
								return fmt.Errorf("node %d does not answer: %w", node, err)
							}
						}
						e.Error = err.Error()
						var ae *canopen.AbortError
						if errors.As(err, &ae) {
							e.Abort = fmt.Sprintf("0x%08X", uint32(ae.Code))
							e.Error = ae.Code.Text()
							if e.Error == "" {
								e.Error = ae.Error()
							}
						}
						res.Failed++
						res.Objects = append(res.Objects, e)
						continue
					}
					timeouts = 0
					res.Read++
					e.Data = fmt.Sprintf("%x", data)
					e.Value, _ = cli.JSONValue(v.DataType, data)
					e.text = cli.ValueText(v.DataType, data)
					res.Objects = append(res.Objects, e)
				}
			}
			if err := a.emit("sdo dump", res); err != nil {
				return err
			}
			if res.Failed > 0 {
				return errs.ErrFindings
			}
			return nil
		},
	}
	bf.add(c, false)
	sf.add(c, true)
	c.Flags().StringVar(&edsPath, "eds", "", "EDS or DCF of the node (required)")
	c.Flags().StringVar(&rng, "range", "", "Only objects in this index range: 6000-6FFF, or one index")
	_ = c.MarkFlagRequired("eds")
	return c
}

// uploadFunc adapts an SDO client to the Upload/Download interfaces of the
// pdo and cia402 packages.
type sdoAdapter struct{ c *sdo.Client }

func (s sdoAdapter) Upload(ctx context.Context, index uint16, sub uint8) ([]byte, error) {
	return s.c.Upload(ctx, index, sub)
}

func (s sdoAdapter) Download(ctx context.Context, index uint16, sub uint8, data []byte) error {
	return s.c.Download(ctx, index, sub, data)
}
