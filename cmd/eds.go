// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/otfabric/canopenctl/internal/cli"
	"github.com/otfabric/canopenctl/internal/errs"
	"github.com/otfabric/canopenctl/internal/output"
	"github.com/otfabric/go-canopen"
	"github.com/otfabric/go-canopen/codegen"
	"github.com/otfabric/go-canopen/eds"
	"github.com/otfabric/go-canopen/master"
	"github.com/otfabric/go-canopen/od"
	"github.com/spf13/cobra"
)

func newEDSCmd(a *app) *cobra.Command {
	c := &cobra.Command{
		Use:   "eds",
		Short: "Validate, show, convert and generate code from EDS and DCF files (CiA 306)",
	}
	c.AddCommand(newEDSValidateCmd(a), newEDSShowCmd(a), newEDSConvertCmd(a), newEDSGenerateCmd(a))
	return c
}

type diagJSON struct {
	Severity string `json:"severity"`
	Section  string `json:"section,omitempty"`
	Key      string `json:"key,omitempty"`
	Message  string `json:"message"`
}

type validateResult struct {
	File        string     `json:"file"`
	DCF         bool       `json:"dcf"`
	Valid       bool       `json:"valid"`
	Errors      int        `json:"errors"`
	Warnings    int        `json:"warnings"`
	Objects     int        `json:"objects"`
	Diagnostics []diagJSON `json:"diagnostics"`
}

func (r validateResult) Text() string {
	var b strings.Builder
	for _, d := range r.Diagnostics {
		where := ""
		if d.Section != "" {
			where = "[" + d.Section + "] "
		}
		if d.Key != "" {
			where += d.Key + ": "
		}
		fmt.Fprintf(&b, "%s: %s%s\n", d.Severity, where, d.Message)
	}
	kind := "EDS"
	if r.DCF {
		kind = "DCF"
	}
	verdict := "valid"
	if !r.Valid {
		verdict = "invalid"
	}
	fmt.Fprintf(&b, "%s: %s %s, %d objects, %d errors, %d warnings", r.File, kind, verdict, r.Objects, r.Errors, r.Warnings)
	return b.String()
}

func (r validateResult) Headers() []string { return []string{"SEVERITY", "SECTION", "KEY", "MESSAGE"} }
func (r validateResult) Rows() [][]string {
	var rows [][]string
	for _, d := range r.Diagnostics {
		rows = append(rows, []string{d.Severity, d.Section, d.Key, d.Message})
	}
	return rows
}

func newEDSValidateCmd(a *app) *cobra.Command {
	var strict bool
	c := &cobra.Command{
		Use:   "validate <file>",
		Short: "Check an EDS or DCF against CiA 306 and CiA 301",
		Long: `validate checks an EDS or DCF: mandatory objects, object lists, object, data
and access types, sub-object counts, default, parameter and limit values
against their data types, PDO mapping flags, PDO counts, compact storage and
DCF commissioning values. Errors (or warnings with --strict) make the exit
code 1; a file that cannot be read at all exits 3.`,
		Example: `  canopenctl eds validate device.eds
  canopenctl eds validate node5.dcf --strict --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			f, err := cli.LoadEDS(args[0])
			if err != nil {
				return err
			}
			r := validateResult{File: args[0], DCF: f.IsDCF(), Objects: len(f.Objects), Diagnostics: []diagJSON{}}
			for _, d := range f.Validate() {
				r.Diagnostics = append(r.Diagnostics, diagJSON{Severity: d.Severity.String(), Section: d.Section, Key: d.Key, Message: d.Message})
				if d.Severity == eds.Error {
					r.Errors++
				} else {
					r.Warnings++
				}
			}
			r.Valid = r.Errors == 0
			if err := a.emit("eds validate", r); err != nil {
				return err
			}
			if r.Errors > 0 || (strict && r.Warnings > 0) {
				return errs.ErrFindings
			}
			return nil
		},
	}
	c.Flags().BoolVar(&strict, "strict", false, "Exit 1 on warnings too")
	return c
}

// objectRow is one sub-object of eds show.
type objectRow struct {
	Index      uint16 `json:"index"`
	SubIndex   uint8  `json:"subIndex"`
	Key        string `json:"key"`
	Object     string `json:"object"`
	Name       string `json:"name"`
	ObjectType string `json:"objectType"`
	Type       string `json:"type"`
	Access     string `json:"access"`
	PDO        bool   `json:"pdoMapping"`
	Default    string `json:"default"`
	Low        string `json:"lowLimit,omitempty"`
	High       string `json:"highLimit,omitempty"`
}

type deviceJSON struct {
	VendorName     string `json:"vendorName"`
	VendorNumber   string `json:"vendorNumber"`
	ProductName    string `json:"productName"`
	ProductNumber  string `json:"productNumber"`
	RevisionNumber string `json:"revisionNumber"`
	OrderCode      string `json:"orderCode,omitempty"`
	BaudRates      []int  `json:"baudRates"`
	RXPDOs         uint16 `json:"rxPDOs"`
	TXPDOs         uint16 `json:"txPDOs"`
	LSS            bool   `json:"lss"`
}

type showResult struct {
	File        string      `json:"file"`
	DCF         bool        `json:"dcf"`
	NodeID      uint8       `json:"nodeId,omitempty"`
	Description string      `json:"description,omitempty"`
	Device      deviceJSON  `json:"device"`
	Objects     []objectRow `json:"objects"`
}

func (r showResult) Text() string {
	var b strings.Builder
	d := r.Device
	fmt.Fprintf(&b, "%s: %s %s (vendor %s %s, product %s, revision %s)\n", r.File, d.ProductName, r.Description, d.VendorName, d.VendorNumber, d.ProductNumber, d.RevisionNumber)
	fmt.Fprintf(&b, "%d RPDOs, %d TPDOs, LSS %v, bit rates %v kbit/s\n", d.RXPDOs, d.TXPDOs, d.LSS, d.BaudRates)
	_ = output.WriteTable(&b, r.Headers(), r.Rows())
	return strings.TrimRight(b.String(), "\n")
}

func (r showResult) Headers() []string {
	return []string{"KEY", "NAME", "TYPE", "ACCESS", "PDO", "DEFAULT", "LIMITS"}
}

func (r showResult) Rows() [][]string {
	var rows [][]string
	for _, o := range r.Objects {
		lim := ""
		if o.Low != "" || o.High != "" {
			lim = o.Low + ".." + o.High
		}
		pdoMap := ""
		if o.PDO {
			pdoMap = "yes"
		}
		rows = append(rows, []string{o.Key, o.Name, o.Type, o.Access, pdoMap, o.Default, lim})
	}
	return rows
}

func newEDSShowCmd(a *app) *cobra.Command {
	var (
		node int
		rng  string
	)
	c := &cobra.Command{
		Use:   "show <file>",
		Short: "List the device information and the objects of an EDS or DCF",
		Long: `show prints the device information and the object table of an EDS or DCF:
every sub-object with its name, data type, access type, PDO mappability,
value (the DCF's ParameterValue, else the default; $NODEID values evaluated
for --node-id when given) and limits. Objects a dictionary cannot hold are
left out (eds validate reports them).`,
		Example: `  canopenctl eds show device.eds
  canopenctl eds show device.eds --range 6000-6FFF --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if node < 0 || node > 127 {
				return errs.Invalid("--node-id %d: 1..127 (0: keep $NODEID)", node)
			}
			lo, hi, err := indexRange(rng)
			if err != nil {
				return err
			}
			f, err := cli.LoadEDS(args[0])
			if err != nil {
				return err
			}
			d, _, err := cli.Dictionary(f, args[0], canopen.NodeID(node), true)
			if err != nil {
				return err
			}
			di := f.DeviceInfo
			r := showResult{File: args[0], DCF: f.IsDCF(), Description: f.FileInfo.Description, Objects: []objectRow{},
				Device: deviceJSON{VendorName: di.VendorName, VendorNumber: cli.Hex(uint64(di.VendorNumber), 8), ProductName: di.ProductName,
					ProductNumber: cli.Hex(uint64(di.ProductNumber), 8), RevisionNumber: cli.Hex(uint64(di.RevisionNumber), 8), OrderCode: di.OrderCode,
					BaudRates: di.SupportedBaudRates(), RXPDOs: di.NrOfRXPDO, TXPDOs: di.NrOfTXPDO, LSS: di.LSSSupported}}
			if r.Device.BaudRates == nil {
				r.Device.BaudRates = []int{}
			}
			if f.DeviceComissioning != nil {
				r.NodeID = f.DeviceComissioning.NodeID
			}
			ds := &desc{path: args[0], file: f, dict: d}
			for _, o := range d.Objects() {
				if o.Index < lo || o.Index > hi {
					continue
				}
				for _, v := range o.Subs {
					row := objectRow{Index: o.Index, SubIndex: v.SubIndex, Key: cli.Key(o.Index, v.SubIndex), Object: o.Name, Name: ds.name(o.Index, v.SubIndex),
						ObjectType: o.Type.String(), Type: v.DataType.String(), Access: v.Access.String(), PDO: v.PDOMappable}
					if node == 0 && v.NodeIDRelative {
						row.Default = eds.FormatLiteral(v.DataType, v.Default, true)
					} else {
						row.Default = eds.FormatLiteral(v.DataType, d.DefaultValue(v), false)
					}
					if v.Low != nil {
						row.Low = eds.FormatLiteral(v.DataType, v.Low, false)
					}
					if v.High != nil {
						row.High = eds.FormatLiteral(v.DataType, v.High, false)
					}
					r.Objects = append(r.Objects, row)
				}
			}
			return a.emit("eds show", r)
		},
	}
	c.Flags().IntVar(&node, "node-id", 0, "Evaluate $NODEID values for this node-ID (0: show them as $NODEID+...)")
	c.Flags().StringVar(&rng, "range", "", "Only objects in this index range: 6000-6FFF, or one index")
	return c
}

type fileResult struct {
	Input  string `json:"input"`
	Output string `json:"output"`
	Kind   string `json:"kind"`
	NodeID uint8  `json:"nodeId,omitempty"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
	note   string
}

func (r fileResult) Text() string {
	s := fmt.Sprintf("%s → %s (%s, %d bytes)", r.Input, r.Output, r.Kind, r.Bytes)
	if r.note != "" {
		s += "; " + r.note
	}
	return s
}

// writeOut writes generated content to path, or to stdout when path is
// "" or "-" (then without a result document).
func (a *app) writeOut(command, path string, content []byte, r fileResult) error {
	if path == "" || path == "-" {
		if _, err := a.out.Write(content); err != nil {
			return errs.New(errs.KindOutput, errs.CodeOutputFailed, "write output: %w", err)
		}
		return nil
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return errs.New(errs.KindOutput, errs.CodeOutputFailed, "write %s: %w", path, err)
	}
	sum := sha256.Sum256(content)
	r.Output, r.Bytes, r.SHA256 = path, len(content), hex.EncodeToString(sum[:])
	return a.emit(command, r)
}

// toDCF turns an EDS (or DCF) into the DCF of a node: commissioning
// section, and every sub-object's value as ParameterValue with $NODEID
// evaluated.
func toDCF(f *eds.File, input, output string, node canopen.NodeID, nodeName string, baud int) {
	if f.DeviceComissioning == nil {
		f.DeviceComissioning = &eds.DeviceComissioning{}
	}
	dc := f.DeviceComissioning
	dc.NodeID = uint8(node)
	if nodeName != "" {
		dc.NodeName = nodeName
	}
	if baud > 0 {
		dc.Baudrate = uint16(baud)
	}
	if !f.IsDCF() || f.FileInfo.LastEDS == "" {
		f.FileInfo.LastEDS = filepath.Base(input)
	}
	if output != "" && output != "-" {
		f.FileInfo.FileName = filepath.Base(output)
	}
	fix := func(e *eds.Entry) {
		if e.DataType == 0 || e.ObjectType == od.ObjectArray || e.ObjectType == od.ObjectRecord {
			return
		}
		lit := e.Value()
		if strings.Contains(strings.ToUpper(lit), "$NODEID") {
			if b, err := canopen.ParseValue(e.DataType, lit, node); err == nil {
				lit = eds.FormatLiteral(e.DataType, b, false)
			}
		}
		e.ParameterValue, e.HasParameterValue = lit, true
	}
	for _, o := range f.Objects {
		fix(&o.Entry)
		for _, s := range o.Subs {
			fix(&s.Entry)
		}
	}
}

// toEDS turns a DCF back into an EDS: no commissioning section, no
// parameter values or denotations.
func toEDS(f *eds.File, output string) {
	f.DeviceComissioning = nil
	f.FileInfo.LastEDS = ""
	if output != "" && output != "-" {
		f.FileInfo.FileName = filepath.Base(output)
	}
	clear := func(e *eds.Entry) {
		e.ParameterValue, e.HasParameterValue, e.Denotation = "", false, ""
	}
	for _, o := range f.Objects {
		clear(&o.Entry)
		o.Values, o.Denotations = nil, nil
		for _, s := range o.Subs {
			clear(&s.Entry)
		}
	}
}

func newEDSConvertCmd(a *app) *cobra.Command {
	var (
		to, out, nodeName string
		node, baud        int
	)
	c := &cobra.Command{
		Use:   "convert <file>",
		Short: "Convert an EDS to a node's DCF, or a DCF back to an EDS",
		Long: `convert writes a DCF for a node from an EDS (or re-targets a DCF to another
node): a [DeviceComissioning] section with --node-id (and --node-name,
--baud), and every sub-object's value as ParameterValue, with $NODEID
evaluated for the node. --to eds removes the commissioning section and the
parameter values. The target format is --to, else the extension of --output
(.dcf, .eds). The output is CiA 306 in a canonical layout (comments are not
kept); without --output it goes to stdout.`,
		Example: `  canopenctl eds convert io401.eds --node-id 5 -o node5.dcf
  canopenctl eds convert node5.dcf --to eds -o io401.eds`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if to == "" {
				to = strings.TrimPrefix(strings.ToLower(filepath.Ext(out)), ".")
			}
			to = strings.ToLower(to)
			if to != "dcf" && to != "eds" {
				return errs.Invalid("give --to dcf or --to eds (or an --output ending in .dcf or .eds)")
			}
			f, err := cli.LoadEDS(args[0])
			if err != nil {
				return err
			}
			var nid canopen.NodeID
			if to == "dcf" {
				if node == 0 && f.DeviceComissioning != nil {
					node = int(f.DeviceComissioning.NodeID)
				}
				if nid, err = cli.ParseNode(strconv.Itoa(node), false); err != nil {
					return errs.Invalid("--node-id %d: a DCF needs the node-ID (1..127)", node)
				}
				if baud < 0 || baud > 1000 {
					return errs.Invalid("--baud %d: kbit/s, 10..1000", baud)
				}
				toDCF(f, args[0], out, nid, nodeName, baud)
			} else {
				toEDS(f, out)
			}
			var buf bytes.Buffer
			if err := eds.Write(&buf, f); err != nil {
				return errs.New(errs.KindInvalidInput, errs.CodeInvalidEDS, "%s: %v", args[0], err)
			}
			note := ""
			if back, err := eds.ParseBytes(buf.Bytes()); err == nil && eds.HasErrors(back.Validate()) {
				note = "the result has validation errors (eds validate)"
				a.warn("%s", note)
			}
			return a.writeOut("eds convert", out, buf.Bytes(), fileResult{Input: args[0], Kind: to, NodeID: uint8(nid), note: note})
		},
	}
	c.Flags().StringVar(&to, "to", "", "Target: dcf or eds (default: from the --output extension)")
	c.Flags().StringVarP(&out, "output", "o", "", "Output file (default: stdout)")
	c.Flags().IntVar(&node, "node-id", 0, "Node-ID of the DCF (default: the input DCF's)")
	c.Flags().StringVar(&nodeName, "node-name", "", "NodeName of the DCF's commissioning section")
	c.Flags().IntVar(&baud, "baud", 0, "Baudrate (kbit/s) of the DCF's commissioning section")
	return c
}

func newEDSGenerateCmd(a *app) *cobra.Command {
	var pkg, out string
	c := &cobra.Command{
		Use:   "generate <file> --package <name>",
		Short: "Generate typed Go code (constants, dictionary, accessors) from an EDS",
		Long: `generate writes a Go source file for an EDS or DCF: index and sub-index
constants, NewDictionary (the object dictionary without parsing at run time),
a typed local accessor and a typed remote accessor over an SDO client. The
output is deterministic and gofmt'd and names the source file and its
SHA-256. The file must validate without errors.`,
		Example: `  canopenctl eds generate io401.eds --package io401 -o io401/io401.go`,
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if !validIdent(pkg) {
				return errs.Invalid("--package %q: a Go package name (lower-case letters, digits, underscores)", pkg)
			}
			src, err := os.ReadFile(args[0])
			if err != nil {
				return errs.New(errs.KindInvalidInput, errs.CodeFileOpen, "cannot read EDS: %w", err)
			}
			f, err := cli.LoadEDS(args[0])
			if err != nil {
				return err
			}
			for _, d := range f.Validate() {
				if d.Severity == eds.Error {
					return errs.New(errs.KindInvalidInput, errs.CodeInvalidEDS, "%s: %s (eds validate lists every problem)", args[0], d)
				}
			}
			sum := sha256.Sum256(src)
			code, err := codegen.Generate(f, codegen.Options{Package: pkg, Source: filepath.Base(args[0]), SourceSHA256: hex.EncodeToString(sum[:])})
			if err != nil {
				return errs.New(errs.KindInvalidInput, errs.CodeInvalidEDS, "%s: %v", args[0], err)
			}
			if out != "" && out != "-" {
				if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
					return errs.New(errs.KindOutput, errs.CodeOutputFailed, "%v", err)
				}
			}
			return a.writeOut("eds generate", out, code, fileResult{Input: args[0], Kind: "go"})
		},
	}
	c.Flags().StringVar(&pkg, "package", "", "Go package name (required)")
	c.Flags().StringVarP(&out, "output", "o", "", "Output file (default: stdout)")
	_ = c.MarkFlagRequired("package")
	return c
}

func validIdent(s string) bool {
	if s == "" || s == "_" || (s[0] >= '0' && s[0] <= '9') {
		return false
	}
	for _, r := range s {
		if r != '_' && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func newDCFCmd(a *app) *cobra.Command {
	c := &cobra.Command{
		Use:   "dcf",
		Short: "Apply a DCF to a node, or export a node's configuration as a DCF",
	}
	c.AddCommand(newDCFApplyCmd(a), newDCFExportCmd(a))
	return c
}

type entryJSON struct {
	Index    uint16 `json:"index"`
	SubIndex uint8  `json:"subIndex"`
	Key      string `json:"key"`
	Name     string `json:"name,omitempty"`
	Data     string `json:"data"`
}

type applyResult struct {
	Node    canopen.NodeID `json:"node"`
	File    string         `json:"file"`
	DryRun  bool           `json:"dryRun"`
	Entries []entryJSON    `json:"entries"`
}

func (r applyResult) Text() string {
	var b strings.Builder
	for _, e := range r.Entries {
		fmt.Fprintf(&b, "%s = %s  %s\n", e.Key, e.Data, e.Name)
	}
	if r.DryRun {
		fmt.Fprintf(&b, "dry run: %d entries of %s would be written to node %d", len(r.Entries), r.File, r.Node)
	} else {
		fmt.Fprintf(&b, "%d entries of %s written to node %d", len(r.Entries), r.File, r.Node)
	}
	return b.String()
}

func (r applyResult) Headers() []string { return []string{"KEY", "NAME", "DATA"} }
func (r applyResult) Rows() [][]string {
	var rows [][]string
	for _, e := range r.Entries {
		rows = append(rows, []string{e.Key, e.Name, e.Data})
	}
	return rows
}

func newDCFApplyCmd(a *app) *cobra.Command {
	var (
		bf     busFlags
		sf     sdoFlags
		dryRun bool
	)
	c := &cobra.Command{
		Use:   "apply <node> <file>",
		Short: "Write a DCF's configuration to a node (needs --yes off virtual interfaces)",
		Long: `apply writes every writable sub-object a DCF configures (ParameterValue,
without the "refuse write on download" flag) to the node by SDO, with
$NODEID values evaluated for the node. PDO parameters are applied per PDO in
the order CiA 301 requires (invalidate, clear the mapping, map, enable, make
valid), however the DCF orders them. The first refused write stops it (exit
11, the abort code and object in the error). --dry-run lists the entries
without sending anything. <node> 0 takes the DCF's node-ID.`,
		Example: `  canopenctl dcf apply 5 node5.dcf -i vcan0
  canopenctl dcf apply 5 node5.dcf --dry-run
  canopenctl dcf apply 0 node5.dcf -i can0 --yes`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			node, err := cli.ParseNode(args[0], true)
			if err != nil {
				return err
			}
			if err := sf.validate(); err != nil {
				return err
			}
			f, err := cli.LoadEDS(args[1])
			if err != nil {
				return err
			}
			if node == 0 {
				if f.DeviceComissioning == nil || !canopen.NodeID(f.DeviceComissioning.NodeID).Valid() {
					return errs.Invalid("node 0: %s has no valid DeviceComissioning NodeID; give the node", args[1])
				}
				node = canopen.NodeID(f.DeviceComissioning.NodeID)
			}
			entries, err := master.DCFEntries(f, node)
			if err != nil {
				return &errs.Error{Kind: errs.KindInvalidInput, Code: errs.CodeInvalidEDS, Message: fmt.Sprintf("%s: %v", args[1], err), Cause: err}
			}
			d, _ := loadDesc(args[1], node)
			r := applyResult{Node: node, File: args[1], DryRun: dryRun, Entries: []entryJSON{}}
			for _, e := range entries {
				r.Entries = append(r.Entries, entryJSON{Index: e.Index, SubIndex: e.SubIndex, Key: cli.Key(e.Index, e.SubIndex), Name: d.name(e.Index, e.SubIndex), Data: hex.EncodeToString(e.Data)})
			}
			if dryRun {
				return a.emit("dcf apply", r)
			}
			if err := bf.check("dcf apply"); err != nil {
				return err
			}
			bus, err := openBus(bf.iface, openOpts{})
			if err != nil {
				return err
			}
			defer func() { _ = bus.Close() }()
			m, err := master.New(bus, master.WithSDOOptions(sf.options()...))
			if err != nil {
				return err
			}
			defer func() { _ = m.Close() }()
			if err := m.Configure(cmd.Context(), node, entries); err != nil {
				return err
			}
			return a.emit("dcf apply", r)
		},
	}
	bf.add(c, true)
	sf.add(c, false)
	c.Flags().BoolVar(&dryRun, "dry-run", false, "List the entries without sending anything")
	return c
}

func newDCFExportCmd(a *app) *cobra.Command {
	var (
		bf           busFlags
		sf           sdoFlags
		edsPath, out string
		nodeName     string
	)
	c := &cobra.Command{
		Use:   "export <node> --eds <file>",
		Short: "Read a node's current values into a DCF (SDO reads)",
		Long: `export reads every readable sub-object the EDS describes from the node and
writes a DCF holding the values read as ParameterValue (objects the node
refuses keep the EDS default and are reported on stderr; the pre-defined
error field 0x1003, diagnostics rather than configuration, keeps it too, so
that dcf apply can write the DCF back). The DCF's
commissioning section gets the node-ID. It only sends SDO read requests.
Without --output the DCF goes to stdout.`,
		Example: `  canopenctl dcf export 5 --eds io401.eds -o node5.dcf -i can0`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			node, err := cli.ParseNode(args[0], false)
			if err != nil {
				return err
			}
			if err := sf.validate(); err != nil {
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
			read, failed := 0, 0
			for _, o := range d.dict.Objects() {
				for _, v := range o.Subs {
					if !v.Access.Readable() || o.Index == od.IndexErrorField {
						continue // the error history is diagnostics, not configuration
					}
					data, err := cl.Upload(cmd.Context(), o.Index, v.SubIndex)
					if err == nil {
						err = d.dict.Set(o.Index, v.SubIndex, data)
					}
					if err != nil {
						if cmd.Context().Err() != nil {
							return err
						}
						if read == 0 && errors.Is(err, canopen.ErrTimeout) {
							return fmt.Errorf("node %d does not answer: %w", node, err)
						}
						failed++
						a.warn("%s: %v (EDS default kept)", cli.Key(o.Index, v.SubIndex), err)
						continue
					}
					read++
				}
			}
			now := time.Now()
			fi := d.file.FileInfo
			fi.LastEDS = filepath.Base(edsPath)
			fi.FileName = filepath.Base(out)
			if out == "" || out == "-" {
				fi.FileName = ""
			}
			fi.ModificationDate, fi.ModificationTime, fi.ModifiedBy = now.Format("01-02-2006"), now.Format("03:04PM"), "canopenctl"
			nf := eds.FromDictionary(d.dict, eds.ExportOptions{FileInfo: fi, DeviceInfo: d.file.DeviceInfo, DCF: true,
				Comissioning: eds.DeviceComissioning{NodeID: uint8(node), NodeName: nodeName}}) //nolint:misspell // CiA 306 spelling
			var buf bytes.Buffer
			if err := eds.Write(&buf, nf); err != nil {
				return errs.New(errs.KindInternal, errs.CodeInternalUnexpected, "write DCF: %v", err)
			}
			note := fmt.Sprintf("%d values read, %d failed", read, failed)
			if out == "" || out == "-" {
				a.warn("%s", note)
			}
			if err := a.writeOut("dcf export", out, buf.Bytes(), fileResult{Input: edsPath, Kind: "dcf", NodeID: uint8(node), note: note}); err != nil {
				return err
			}
			if failed > 0 {
				return errs.ErrFindings
			}
			return nil
		},
	}
	bf.add(c, false)
	sf.add(c, true)
	c.Flags().StringVar(&edsPath, "eds", "", "EDS of the node (required)")
	c.Flags().StringVarP(&out, "output", "o", "", "Output file (default: stdout)")
	c.Flags().StringVar(&nodeName, "node-name", "", "NodeName of the DCF's commissioning section")
	_ = c.MarkFlagRequired("eds")
	return c
}
