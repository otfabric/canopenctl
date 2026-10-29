// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/otfabric/canopenctl/internal/cli"
	"github.com/otfabric/canopenctl/internal/errs"
	"github.com/otfabric/go-canopen"
	"github.com/otfabric/go-canopen/cdcf"
	"github.com/otfabric/go-canopen/lss"
	"github.com/otfabric/go-canopen/master"
	"github.com/otfabric/go-canopen/od"
	"github.com/otfabric/go-canopen/program"
	"github.com/otfabric/go-canopen/store"
	"github.com/spf13/cobra"
)

// loadConcise reads a node configuration: a concise DCF (binary, CiA
// 302-3) or a DCF/EDS (text), whose configuration is turned into a
// concise DCF for node (0: the DCF's node-ID) with the PDO sequence of
// CiA 301. kind says which it was.
func loadConcise(path string, node canopen.NodeID) (entries []cdcf.Entry, kind string, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, "", errs.New(errs.KindInvalidInput, errs.CodeFileOpen, "cannot read %s: %w", path, err)
	}
	t := bytes.TrimLeft(b, "\xef\xbb\xbf \t\r\n")
	if len(t) > 0 && (t[0] == '[' || t[0] == ';') {
		f, err := cli.LoadEDS(path)
		if err != nil {
			return nil, "", err
		}
		e, err := cdcf.FromFile(f, node)
		if err != nil {
			return nil, "", &errs.Error{Kind: errs.KindInvalidInput, Code: errs.CodeInvalidEDS, Message: fmt.Sprintf("%s: %v", path, err), Cause: err}
		}
		return e, "dcf", nil
	}
	e, err := cdcf.Decode(b)
	if err != nil {
		return nil, "", errs.New(errs.KindInvalidInput, errs.CodeInvalidValue, "%s is neither a DCF nor a concise DCF: %w", path, err)
	}
	return e, "cdcf", nil
}

func entriesJSON(entries []cdcf.Entry, d *desc) []entryJSON {
	out := []entryJSON{}
	for _, e := range entries {
		out = append(out, entryJSON{Index: e.Index, SubIndex: e.SubIndex, Key: cli.Key(e.Index, e.SubIndex), Name: d.name(e.Index, e.SubIndex),
			Data: hex.EncodeToString(e.Data)})
	}
	return out
}

func newCDCFCmd(a *app) *cobra.Command {
	c := &cobra.Command{
		Use:   "cdcf",
		Short: "Concise DCFs (CiA 302-3): convert a DCF, inspect, apply to a node",
		Long: `A concise DCF is the binary node configuration a CiA 302 manager keeps in
0x1F22 and writes to the node at boot-up: a count, then entries of index,
sub-index, size and data, written in their order. convert makes one from a
DCF (with the PDO parameters in the sequence CiA 301 requires), show lists
one, apply writes one (or a DCF) to a node.`,
	}
	c.AddCommand(newCDCFConvertCmd(a), newCDCFShowCmd(a), newCDCFApplyCmd(a))
	return c
}

func newCDCFConvertCmd(a *app) *cobra.Command {
	var (
		nodeID  int
		outPath string
	)
	c := &cobra.Command{
		Use:   "convert <dcf>",
		Short: "Convert a DCF into a concise DCF",
		Long: `convert extracts the configuration of a DCF (every writable sub-object with a
ParameterValue, without the "refuse write on download" flag, $NODEID
evaluated for --node-id or the DCF's node-ID) as a concise DCF, PDO
parameters in the CiA 301 sequence (invalidate, clear the mapping, map,
enable, make valid). --output writes it to a file (the result describes
it); without it the binary goes to stdout.`,
		Example: `  canopenctl cdcf convert node5.dcf -o node5.cdcf
  canopenctl cdcf convert device.eds --node-id 5 -o node5.cdcf`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if nodeID < 0 || nodeID > 127 {
				return errs.Invalid("--node-id %d: 1..127", nodeID)
			}
			f, err := cli.LoadEDS(args[0])
			if err != nil {
				return err
			}
			entries, err := cdcf.FromFile(f, canopen.NodeID(nodeID))
			if err != nil {
				return &errs.Error{Kind: errs.KindInvalidInput, Code: errs.CodeInvalidEDS, Message: fmt.Sprintf("%s: %v", args[0], err), Cause: err}
			}
			b, err := cdcf.Encode(entries)
			if err != nil {
				return &errs.Error{Kind: errs.KindInvalidInput, Code: errs.CodeInvalidEDS, Message: fmt.Sprintf("%s: %v", args[0], err), Cause: err}
			}
			n := uint8(nodeID)
			if n == 0 && f.DeviceComissioning != nil {
				n = f.DeviceComissioning.NodeID
			}
			return a.writeOut("cdcf convert", outPath, b, fileResult{Input: args[0], Kind: "cdcf", NodeID: n,
				note: fmt.Sprintf("%d entries", len(entries))})
		},
	}
	c.Flags().IntVar(&nodeID, "node-id", 0, "Node-ID for $NODEID (default: the DCF's)")
	c.Flags().StringVarP(&outPath, "output", "o", "", "Write the concise DCF to this file (default: stdout)")
	return c
}

type cdcfShowResult struct {
	File    string      `json:"file"`
	Format  string      `json:"format"`
	Size    int         `json:"size"`
	Entries []entryJSON `json:"entries"`
}

func (r cdcfShowResult) Text() string {
	var b strings.Builder
	for _, e := range r.Entries {
		fmt.Fprintf(&b, "%s = %s  %s\n", e.Key, e.Data, e.Name)
	}
	fmt.Fprintf(&b, "%s: %d entries, %d bytes as a concise DCF", r.File, len(r.Entries), r.Size)
	return b.String()
}

func (r cdcfShowResult) Headers() []string { return []string{"KEY", "NAME", "DATA"} }
func (r cdcfShowResult) Rows() [][]string  { return applyResult{Entries: r.Entries}.Rows() }

func newCDCFShowCmd(a *app) *cobra.Command {
	var (
		nodeID  int
		edsPath string
	)
	c := &cobra.Command{
		Use:   "show <file>",
		Short: "List the entries of a concise DCF (or of the concise DCF of a DCF)",
		Long: `show decodes a concise DCF and lists its entries in order (object, data);
--eds names the objects. Given a DCF it shows the concise DCF that convert
would write.`,
		Example: `  canopenctl cdcf show node5.cdcf --eds device.eds
  canopenctl cdcf show node5.dcf --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if nodeID < 0 || nodeID > 127 {
				return errs.Invalid("--node-id %d: 1..127", nodeID)
			}
			entries, kind, err := loadConcise(args[0], canopen.NodeID(nodeID))
			if err != nil {
				return err
			}
			names := edsPath
			if names == "" && kind == "dcf" {
				names = args[0]
			}
			d, err := loadDesc(names, 1)
			if err != nil {
				return err
			}
			return a.emit("cdcf show", cdcfShowResult{File: args[0], Format: kind, Size: cdcf.Size(entries), Entries: entriesJSON(entries, d)})
		},
	}
	c.Flags().IntVar(&nodeID, "node-id", 0, "Node-ID for $NODEID of a DCF (default: the DCF's)")
	c.Flags().StringVar(&edsPath, "eds", "", "EDS or DCF naming the objects")
	return c
}

func newCDCFApplyCmd(a *app) *cobra.Command {
	var (
		bf      busFlags
		sf      sdoFlags
		edsPath string
		dryRun  bool
	)
	c := &cobra.Command{
		Use:   "apply <node> <file>",
		Short: "Write a concise DCF (or a DCF) to a node, entry by entry (needs --yes off virtual interfaces)",
		Long: `apply writes the entries of a concise DCF to a node one by one, in their order,
as a CiA 302 manager does (nothing is regrouped: the file must hold the
sequence the node needs; convert writes PDOs so). A DCF is converted first.
The first refused entry stops it (exit 11, the abort code and object in the
error). --dry-run lists the entries without sending anything.`,
		Example: `  canopenctl cdcf apply 5 node5.cdcf -i vcan0
  canopenctl cdcf apply 5 node5.dcf --dry-run`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			node, err := cli.ParseNode(args[0], false)
			if err != nil {
				return err
			}
			if err := sf.validate(); err != nil {
				return err
			}
			if err := bf.checkFD(); err != nil {
				return err
			}
			entries, kind, err := loadConcise(args[1], node)
			if err != nil {
				return err
			}
			names := edsPath
			if names == "" && kind == "dcf" {
				names = args[1]
			}
			d, err := loadDesc(names, node)
			if err != nil {
				return err
			}
			r := applyResult{Node: node, File: args[1], DryRun: dryRun, Entries: entriesJSON(entries, d)}
			if dryRun {
				return a.emit("cdcf apply", r)
			}
			if err := bf.check("cdcf apply"); err != nil {
				return err
			}
			bus, err := bf.open(false)
			if err != nil {
				return err
			}
			defer func() { _ = bus.Close() }()
			m, err := master.New(bus, sf.masterOptions(&bf)...)
			if err != nil {
				return err
			}
			defer func() { _ = m.Close() }()
			if err := m.ConfigureConcise(cmd.Context(), node, entries); err != nil {
				return err
			}
			return a.emit("cdcf apply", r)
		},
	}
	bf.add(c, true)
	bf.addFD(c, "write over USDO instead of SDO")
	sf.add(c, false)
	sf.addFD(c)
	c.Flags().StringVar(&edsPath, "eds", "", "EDS or DCF naming the objects")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "List the entries without sending anything")
	return c
}

// groupNames names the parameter groups of 0x1010/0x1011.
var groupNames = map[uint8]string{store.All: "all", store.Communication: "communication", store.Application: "application",
	store.Manufacturer: "manufacturer"}

func groupName(g uint8) string {
	if n, ok := groupNames[g]; ok {
		return n
	}
	return "sub-index " + strconv.Itoa(int(g))
}

type storeResult struct {
	Node   canopen.NodeID `json:"node"`
	Action string         `json:"action"`
	Group  uint8          `json:"group"`
	Name   string         `json:"groupName"`
	Key    string         `json:"key"`
	Reset  bool           `json:"reset,omitempty"`
}

func (r storeResult) Text() string {
	if r.Action == "store" {
		return fmt.Sprintf("node %d saved its %s parameters (%s = \"save\")", r.Node, r.Name, r.Key)
	}
	s := fmt.Sprintf("node %d restores the defaults of its %s parameters at its next reset (%s = \"load\")", r.Node, r.Name, r.Key)
	if r.Reset {
		s += "; NMT reset node sent"
	}
	return s
}

func newStoreCmd(a *app, restore bool) *cobra.Command {
	var (
		bf    busFlags
		sf    sdoFlags
		group string
		reset bool
	)
	use, what := "store", "store"
	short := "Make a node save its parameters (0x1010, needs --yes off virtual interfaces)"
	long := `store writes the signature "save" to 0x1010 of a node: the node saves the
current values of a parameter group (--group all, communication,
application, manufacturer, or a sub-index) in non-volatile memory and loads
them at its next resets. A device that cannot save refuses (exit 11).`
	example := `  canopenctl store 5 -i vcan0
  canopenctl store 5 --group communication -i can0 --yes`
	if restore {
		use, what = "restore", "restore"
		short = "Make a node restore its default parameters (0x1011, needs --yes off virtual interfaces)"
		long = `restore writes the signature "load" to 0x1011 of a node: the node discards the
saved values of a parameter group (--group all, communication, application,
manufacturer, or a sub-index) and has its default values from its next
reset on. --reset sends that reset (NMT reset node) at once.`
		example = `  canopenctl restore 5 --reset -i vcan0
  canopenctl restore 5 --group application -i can0 --yes`
	}
	c := &cobra.Command{
		Use:     use + " <node>",
		Short:   short,
		Long:    long,
		Example: example,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			node, err := cli.ParseNode(args[0], false)
			if err != nil {
				return err
			}
			g, err := cli.ParseStoreGroup(group)
			if err != nil {
				return err
			}
			if err := sf.validate(); err != nil {
				return err
			}
			if err := bf.checkFD(); err != nil {
				return err
			}
			if err := bf.check(what); err != nil {
				return err
			}
			bus, err := bf.open(false)
			if err != nil {
				return err
			}
			defer func() { _ = bus.Close() }()
			m, err := master.New(bus, sf.masterOptions(&bf)...)
			if err != nil {
				return err
			}
			defer func() { _ = m.Close() }()
			r := storeResult{Node: node, Action: what, Group: g, Name: groupName(g)}
			if restore {
				r.Key = cli.Key(store.IndexRestore, g)
				err = m.RestoreDefaults(cmd.Context(), node, g)
				if err == nil && reset {
					err = m.ResetNode(cmd.Context(), node)
					r.Reset = true
				}
			} else {
				r.Key = cli.Key(store.IndexStore, g)
				err = m.StoreParameters(cmd.Context(), node, g)
			}
			if err != nil {
				return err
			}
			return a.emit(what, r)
		},
	}
	bf.add(c, true)
	bf.addFD(c, "write over USDO instead of SDO")
	sf.add(c, false)
	sf.addFD(c)
	c.Flags().StringVar(&group, "group", "all", "Parameter group: all, communication, application, manufacturer, or a sub-index")
	if restore {
		c.Flags().BoolVar(&reset, "reset", false, "Reset the node (NMT reset node) so that the defaults apply now")
	}
	return c
}

func newProgramCmd(a *app) *cobra.Command {
	c := &cobra.Command{
		Use:   "program",
		Short: "Program download (CiA 302-3): replace a node's program, control and read its state",
		Long: `The program commands use the program download objects of a device: program
data 0x1F50 (the image), program control 0x1F51 (stop, start, reset,
clear; the state), application software identification 0x1F56 and flash
status 0x1F57. Program control commands are accepted in pre-operational.`,
	}
	c.AddCommand(newProgramDownloadCmd(a), newProgramControlCmd(a), newProgramStatusCmd(a))
	return c
}

type programResult struct {
	Node           canopen.NodeID `json:"node"`
	Program        uint8          `json:"program"`
	Action         string         `json:"action"`
	File           string         `json:"file,omitempty"`
	Bytes          int            `json:"bytes,omitempty"`
	Identification string         `json:"identification,omitempty"`
	State          string         `json:"state,omitempty"`
	StateValue     *uint8         `json:"stateValue,omitempty"`
	FlashStatus    string         `json:"flashStatus,omitempty"`
	FlashText      string         `json:"flashText,omitempty"`
	ElapsedMs      int64          `json:"elapsedMs,omitempty"`
}

func (r programResult) Text() string {
	switch r.Action {
	case "download":
		s := fmt.Sprintf("node %d program %d: %d bytes of %s downloaded and started in %d ms", r.Node, r.Program, r.Bytes, r.File, r.ElapsedMs)
		if r.Identification != "" {
			s += ", identification " + r.Identification
		}
		return s
	case "status":
		return fmt.Sprintf("node %d program %d: %s, identification %s, flash status %s (%s)", r.Node, r.Program, r.State, r.Identification,
			r.FlashStatus, r.FlashText)
	}
	return fmt.Sprintf("node %d program %d: %s sent", r.Node, r.Program, r.Action)
}

// programFlags are the flags shared by the program commands.
type programFlags struct {
	bf  busFlags
	sf  sdoFlags
	num int
}

func (p *programFlags) add(c *cobra.Command, transmits bool) {
	p.bf.add(c, transmits)
	p.bf.addFD(c, "use USDO instead of SDO (bulk transfer of the image)")
	p.sf.add(c, false)
	p.sf.addFD(c)
	c.Flags().IntVar(&p.num, "program", 1, "Program number: the sub-index of 0x1F50/0x1F51 (1..254)")
}

func (p *programFlags) validate() error {
	if p.num < 1 || p.num > 254 {
		return errs.Invalid("--program %d: 1..254", p.num)
	}
	if err := p.sf.validate(); err != nil {
		return err
	}
	return p.bf.checkFD()
}

// open opens the bus and a master; done closes both.
func (p *programFlags) open(extra ...master.Option) (*master.Master, func(), error) {
	bus, err := p.bf.open(false)
	if err != nil {
		return nil, nil, err
	}
	m, err := master.New(bus, append(p.sf.masterOptions(&p.bf), extra...)...)
	if err != nil {
		_ = bus.Close()
		return nil, nil, err
	}
	return m, func() { _ = m.Close(); _ = bus.Close() }, nil
}

func newProgramDownloadCmd(a *app) *cobra.Command {
	var (
		pf      programFlags
		ident   string
		useCRC  bool
		poll    time.Duration
		timeout time.Duration
	)
	c := &cobra.Command{
		Use:   "download <node> <file>",
		Short: "Replace the program of a node (needs --yes off virtual interfaces)",
		Long: `download replaces the program of a node as a CiA 302 manager does: stop the
program (0x1F51 = 0), clear it (3), write the image to 0x1F50 (SDO block
transfer, segmented when the device refuses block; USDO bulk with --fd),
poll the flash status 0x1F57 until programming ends, compare the
application software identification 0x1F56 with --identification (or the
CRC-32 of the file with --crc32), start the program (1) and wait until
0x1F51 reads started. The node must be pre-operational. A flash error or a
wrong identification is exit 12 (program_failed); --max-time bounds the
whole download, --timeout each SDO transfer.`,
		Example: `  canopenctl program download 5 firmware.bin --crc32 -i vcan0
  canopenctl program download 5 firmware.bin --identification 0x01020304 -i can0 --yes`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			node, err := cli.ParseNode(args[0], false)
			if err != nil {
				return err
			}
			if err := pf.validate(); err != nil {
				return err
			}
			if ident != "" && useCRC {
				return errs.Invalid("give --identification or --crc32, not both")
			}
			if poll <= 0 || timeout <= 0 {
				return errs.Invalid("--poll and --max-time: more than 0")
			}
			var want uint32
			if ident != "" {
				if want, err = cli.ParseUint32(ident); err != nil {
					return err
				}
			}
			data, err := os.ReadFile(args[1])
			if err != nil {
				return errs.New(errs.KindInvalidInput, errs.CodeFileOpen, "cannot read %s: %w", args[1], err)
			}
			if len(data) == 0 {
				return errs.Invalid("%s is empty", args[1])
			}
			if useCRC {
				want = crc32.ChecksumIEEE(data)
			}
			if err := pf.bf.check("program download"); err != nil {
				return err
			}
			m, done, err := pf.open()
			if err != nil {
				return err
			}
			defer done()
			since := time.Now()
			err = m.DownloadProgram(cmd.Context(), node, data, master.ProgramOptions{Program: uint8(pf.num), Identification: want,
				Timing: master.BootTiming{PollInterval: poll, ProgramTimeout: timeout}})
			if err != nil {
				return err
			}
			r := programResult{Node: node, Program: uint8(pf.num), Action: "download", File: args[1], Bytes: len(data),
				State: program.Started.String(), ElapsedMs: time.Since(since).Milliseconds()}
			if want != 0 {
				r.Identification = cli.Hex(uint64(want), 8)
			}
			return a.emit("program download", r)
		},
	}
	pf.add(c, true)
	c.Flags().StringVar(&ident, "identification", "", "Expected application software identification 0x1F56 after the download")
	c.Flags().BoolVar(&useCRC, "crc32", false, "Expect the CRC-32 (IEEE) of the file as identification 0x1F56")
	c.Flags().DurationVar(&poll, "poll", 100*time.Millisecond, "Polling interval of the flash status and program control")
	c.Flags().DurationVar(&timeout, "max-time", 5*time.Minute, "Bound of the whole download")
	return c
}

// programCommands are the program control commands by name.
var programCommands = map[string]program.Command{"stop": program.Stop, "start": program.Start, "reset": program.Reset, "clear": program.Clear}

func newProgramControlCmd(a *app) *cobra.Command {
	var pf programFlags
	c := &cobra.Command{
		Use:   "control <node> <stop|start|reset|clear>",
		Short: "Write a program control command to 0x1F51 (needs --yes off virtual interfaces)",
		Long: `control writes a command to the program control 0x1F51 of a node: stop (0),
start (1), reset (2) or clear (3). The device accepts them in
pre-operational and refuses others with an abort (exit 11).`,
		Example: `  canopenctl program control 5 stop -i vcan0`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			node, err := cli.ParseNode(args[0], false)
			if err != nil {
				return err
			}
			pc, ok := programCommands[strings.ToLower(args[1])]
			if !ok {
				return errs.Invalid("unknown program command %q (stop, start, reset, clear)", args[1])
			}
			if err := pf.validate(); err != nil {
				return err
			}
			if err := pf.bf.check("program control"); err != nil {
				return err
			}
			bus, err := pf.bf.open(false)
			if err != nil {
				return err
			}
			defer func() { _ = bus.Close() }()
			cl, err := newRemote(bus, node, &pf.sf, &pf.bf)
			if err != nil {
				return err
			}
			defer func() { _ = cl.Close() }()
			if err := cl.Download(cmd.Context(), program.IndexControl, uint8(pf.num), []byte{byte(pc)}); err != nil {
				return err
			}
			return a.emit("program control", programResult{Node: node, Program: uint8(pf.num), Action: pc.String()})
		},
	}
	pf.add(c, true)
	return c
}

func newProgramStatusCmd(a *app) *cobra.Command {
	var pf programFlags
	c := &cobra.Command{
		Use:   "status <node>",
		Short: "Read the program state, identification and flash status of a node (SDO reads)",
		Long: `status reads the program control 0x1F51 (the program's state), the
application software identification 0x1F56 and the flash status 0x1F57 of a
node. It only sends read requests.`,
		Example: `  canopenctl program status 5 -i vcan0`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			node, err := cli.ParseNode(args[0], false)
			if err != nil {
				return err
			}
			if err := pf.validate(); err != nil {
				return err
			}
			bus, err := pf.bf.open(false)
			if err != nil {
				return err
			}
			defer func() { _ = bus.Close() }()
			cl, err := newRemote(bus, node, &pf.sf, &pf.bf)
			if err != nil {
				return err
			}
			defer func() { _ = cl.Close() }()
			ctx := cmd.Context()
			n := uint8(pf.num)
			st, err := cl.Upload(ctx, program.IndexControl, n)
			if err != nil {
				return err
			}
			id, err := cl.Upload(ctx, program.IndexIdentification, n)
			if err != nil {
				return err
			}
			fl, err := cl.Upload(ctx, program.IndexFlashStatus, n)
			if err != nil {
				return err
			}
			if len(st) != 1 || len(id) != 4 || len(fl) != 4 {
				return errs.New(errs.KindProtocol, errs.CodeDataType, "node %d: unexpected sizes of 0x1F51/0x1F56/0x1F57: %d, %d, %d bytes", node, len(st), len(id), len(fl))
			}
			fs := program.FlashStatus(binary.LittleEndian.Uint32(fl))
			flash := "ok"
			switch {
			case fs.InProgress():
				flash = "programming in progress"
			case fs.Err() != nil:
				flash = fs.Err().Error()
			}
			v := st[0]
			return a.emit("program status", programResult{Node: node, Program: n, Action: "status", State: program.State(v).String(), StateValue: &v,
				Identification: cli.Hex(uint64(binary.LittleEndian.Uint32(id)), 8), FlashStatus: cli.Hex(uint64(fs), 8), FlashText: flash})
		},
	}
	pf.add(c, false)
	return c
}

// commonParts returns the parts of the LSS address that every expected
// identity gives with the same non-zero value: the LSS step's Fastscan
// verifies them instead of scanning them.
func commonParts(expected []lss.Address) map[lss.Part]uint32 {
	known := map[lss.Part]uint32{}
	for p := lss.PartVendor; p <= lss.PartRevision; p++ {
		v := uint32(0)
		for i, a := range expected {
			if i == 0 {
				v = a.Part(p)
			} else if a.Part(p) != v {
				v = 0
				break
			}
		}
		if v != 0 {
			known[p] = v
		}
	}
	return known
}

// bootSlave is one slave of boot.
type bootSlave struct {
	Node      canopen.NodeID `json:"node"`
	Mandatory bool           `json:"mandatory"`
	Config    string         `json:"config,omitempty"`
	Entries   int            `json:"entries"`
	Expected  *addressJSON   `json:"expected,omitempty"`
	Booted    bool           `json:"booted"`
	Status    string         `json:"status"`
	Code      string         `json:"code,omitempty"`
	Error     string         `json:"error,omitempty"`
	LSS       *addressJSON   `json:"lss,omitempty"`
}

type bootResult struct {
	Master    canopen.NodeID `json:"master"`
	LSS       bool           `json:"lss"`
	Started   bool           `json:"started"`
	Slaves    []bootSlave    `json:"slaves"`
	Booted    int            `json:"booted"`
	Failed    int            `json:"failed"`
	Error     string         `json:"error,omitempty"`
	ElapsedMs int64          `json:"elapsedMs"`
}

func (r bootResult) Text() string {
	var b strings.Builder
	for _, s := range r.Slaves {
		kind := "optional"
		if s.Mandatory {
			kind = "mandatory"
		}
		fmt.Fprintf(&b, "node %3d  %-9s  %-8s  %s", s.Node, kind, map[bool]string{true: "booted", false: "failed"}[s.Booted], s.Status)
		if s.Code != "" {
			fmt.Fprintf(&b, " (%s)", s.Code)
		}
		if s.Entries > 0 {
			fmt.Fprintf(&b, ", %d entries of %s", s.Entries, s.Config)
		}
		if s.LSS != nil {
			fmt.Fprintf(&b, ", node-ID given by LSS to %s", s.LSS.Address)
		}
		if s.Error != "" {
			fmt.Fprintf(&b, ": %s", s.Error)
		}
		b.WriteString("\n")
	}
	if r.Error != "" {
		fmt.Fprintf(&b, "error: %s\n", r.Error)
	}
	fmt.Fprintf(&b, "network boot-up: %d of %d slaves booted in %d ms", r.Booted, len(r.Slaves), r.ElapsedMs)
	return b.String()
}

func (r bootResult) Headers() []string {
	return []string{"NODE", "MANDATORY", "BOOTED", "STATUS", "ENTRIES", "LSS"}
}

func (r bootResult) Rows() [][]string {
	var rows [][]string
	for _, s := range r.Slaves {
		l := ""
		if s.LSS != nil {
			l = s.LSS.Address
		}
		rows = append(rows, []string{strconv.Itoa(int(s.Node)), strconv.FormatBool(s.Mandatory), strconv.FormatBool(s.Booted), s.Status,
			strconv.Itoa(s.Entries), l})
	}
	return rows
}

// bootEvents collects the master's events of a boot.
type bootEvents struct {
	mu     sync.Mutex
	failed map[canopen.NodeID]error
	lss    map[canopen.NodeID]lss.Address
	done   chan error // the end of the network boot-up: nil, or its failure
}

func (e *bootEvents) add(ev master.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	switch ev.Kind {
	case master.EventBootFailed:
		e.failed[ev.Node] = ev.Err
	case master.EventLSSAssigned:
		if ev.LSS != nil {
			e.lss[ev.Node] = ev.LSS.Address
		}
	case master.EventNetworkBootDone, master.EventNetworkBootFailed:
		err := ev.Err
		if ev.Kind == master.EventNetworkBootFailed && err == nil {
			err = fmt.Errorf("network boot-up failed at node %d", ev.Node)
		}
		select {
		case e.done <- err:
		default:
		}
	default:
	}
}

func newBootCmd(a *app) *cobra.Command {
	var (
		bf        busFlags
		sf        sdoFlags
		slaves    []string
		expects   []string
		optional  string
		masterID  int
		lssStep   bool
		noStart   bool
		storeCfg  bool
		bootTime  time.Duration
		timeout   time.Duration
		lssWindow time.Duration
	)
	c := &cobra.Command{
		Use:   "boot --slave <node>[=<config>]...",
		Short: "Boot a network as a CiA 302 manager: identity checks, configuration, start (needs --yes off virtual interfaces)",
		Long: `boot runs the network boot-up of CiA 302-2 as an NMT master (go-canopen's
manager on a node of its own, --master-id): it resets the slaves (--slave,
repeatable: a node-ID, with =FILE its configuration, a DCF or a concise
DCF), then for each slave when its boot-up arrives: checks the identity
(--expect NODE=vendor:product:revision:serial, hexadecimal, 0 parts not
checked), writes the configuration (the concise DCF, in order; --store makes
the slave save it), starts the error control and, unless --no-start, starts
the slaves (NMT start). Slaves are mandatory unless listed in --optional:
the boot-up waits up to --boot-time for each mandatory slave and fails
without it; optional slaves that do not boot are reported, they do not fail
it.

--lss adds the LSS step first: the devices without node-ID are found with
Fastscan and each is given the node-ID of the slave whose --expect identity
it has (at least one part must be given); they then boot like the others.
Vendor-ID, product code and revision that all --expect identities share are
verified rather than scanned, which makes the Fastscan faster.

The result lists every slave with its CiA 302 boot status (A..O, as 0x1F82
reports them); a slave that failed makes the exit code 1.`,
		Example: `  canopenctl boot --slave 5=node5.dcf --slave 6=node6.cdcf -i vcan0
  canopenctl boot --lss --slave 5=node5.dcf --expect 5=F7A:C0C7:0:1001 --slave 6 --expect 6=F7A:C0C7:0:1002 -i can0 --yes`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !canopen.NodeID(masterID).Valid() {
				return errs.Invalid("--master-id %d: 1..127", masterID)
			}
			if timeout <= 0 || bootTime < 0 || bootTime > time.Duration(1<<32-1)*time.Millisecond || lssWindow <= 0 {
				return errs.Invalid("--max-time and --lss-window more than 0, --boot-time 0 or more")
			}
			if err := sf.validate(); err != nil {
				return err
			}
			if err := bf.checkFD(); err != nil {
				return err
			}
			if len(slaves) == 0 {
				return errs.Invalid("give at least one --slave")
			}
			opt := map[canopen.NodeID]bool{}
			if optional != "" {
				list, err := cli.ParseNodeList(optional)
				if err != nil {
					return err
				}
				for _, n := range list {
					opt[n] = true
				}
			}
			type slaveCfg struct {
				entries []cdcf.Entry
				file    string
				expect  *lss.Address
			}
			cfgs := map[canopen.NodeID]*slaveCfg{}
			for _, s := range slaves {
				id, file := canopen.NodeID(0), ""
				if strings.Contains(s, "=") {
					var err error
					if id, file, err = cli.ParseNodeValue(s); err != nil {
						return err
					}
				} else {
					var err error
					if id, err = cli.ParseNode(s, false); err != nil {
						return err
					}
				}
				if id == canopen.NodeID(masterID) {
					return errs.Invalid("--slave %d is the master's node-ID (--master-id)", id)
				}
				if cfgs[id] != nil {
					return errs.Invalid("--slave %d given twice", id)
				}
				sc := &slaveCfg{file: file}
				if file != "" {
					entries, _, err := loadConcise(file, id)
					if err != nil {
						return err
					}
					sc.entries = entries
				}
				cfgs[id] = sc
			}
			for n := range opt {
				if cfgs[n] == nil {
					return errs.Invalid("--optional %d is not a --slave", n)
				}
			}
			for _, e := range expects {
				id, v, err := cli.ParseNodeValue(e)
				if err != nil {
					return err
				}
				addr, err := cli.ParseLSSAddress(v)
				if err != nil {
					return err
				}
				if cfgs[id] == nil {
					return errs.Invalid("--expect %d is not a --slave", id)
				}
				cfgs[id].expect = &addr
			}
			var expected []lss.Address
			if lssStep {
				for id, sc := range cfgs {
					if sc.expect == nil || *sc.expect == (lss.Address{}) {
						return errs.Invalid("--lss: slave %d needs a non-zero --expect identity", id)
					}
					expected = append(expected, *sc.expect)
				}
			}
			// The manager's dictionary.
			md, err := od.Standard(od.Config{NodeID: canopen.NodeID(masterID), DeviceName: "canopenctl CiA 302 manager", HeartbeatConsumers: 4})
			if err != nil {
				return errs.New(errs.KindInternal, errs.CodeInternalUnexpected, "manager dictionary: %v", err)
			}
			if err := master.AddManagerObjects(md); err != nil {
				return errs.New(errs.KindInternal, errs.CodeInternalUnexpected, "manager objects: %v", err)
			}
			ids := make([]canopen.NodeID, 0, len(cfgs))
			for id := range cfgs {
				ids = append(ids, id)
			}
			sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
			for _, id := range ids {
				sc := cfgs[id]
				flags := master.AssignSlave | master.AssignBoot
				if !opt[id] {
					flags |= master.AssignMandatory
				}
				set := func(index uint16, v uint64) error { return md.SetUint(index, uint8(id), v) }
				err := set(master.IndexSlaveAssignment, uint64(master.NewAssignment(flags, 0, 0)))
				if err == nil && sc.expect != nil {
					x := sc.expect
					for _, kv := range []struct {
						index uint16
						v     uint32
					}{{master.IndexExpectedVendorID, x.Vendor}, {master.IndexExpectedProductCode, x.Product},
						{master.IndexExpectedRevision, x.Revision}, {master.IndexExpectedSerialNumber, x.Serial}} {
						if err == nil {
							err = set(kv.index, uint64(kv.v))
						}
					}
				}
				if err == nil && len(sc.entries) > 0 {
					var b []byte
					if b, err = cdcf.Encode(sc.entries); err == nil {
						err = md.Set(cdcf.IndexConciseDCF, uint8(id), b)
					}
				}
				if err != nil {
					return errs.Invalid("slave %d: %v", id, err)
				}
			}
			if err := md.SetUint(master.IndexBootTime, 0, uint64(bootTime.Milliseconds())); err != nil {
				return errs.New(errs.KindInternal, errs.CodeInternalUnexpected, "boot time: %v", err)
			}
			// The network boot-up runs at the boot-up of the manager's
			// node, at Start.
			startup := master.StartupMaster
			if noStart {
				startup |= master.StartupNoStartSlaves
			}
			if err := md.SetUint(master.IndexNMTStartup, 0, uint64(startup)); err != nil {
				return errs.New(errs.KindInternal, errs.CodeInternalUnexpected, "NMT startup: %v", err)
			}
			if err := bf.check("boot"); err != nil {
				return err
			}
			bus, err := bf.open(false)
			if err != nil {
				return err
			}
			defer func() { _ = bus.Close() }()
			ev := &bootEvents{failed: map[canopen.NodeID]error{}, lss: map[canopen.NodeID]lss.Address{}, done: make(chan error, 1)}
			opts := append(sf.masterOptions(&bf), master.WithDictionary(md), master.WithEventHandler(ev.add),
				master.WithLSSOptions(lss.WithFastscanTimeout(lssWindow)))
			if bf.fd {
				opts = append(opts, master.WithUSDOClient(canopen.NodeID(masterID), sf.usdoOptions(bf.brs)...))
			}
			if lssStep {
				opts = append(opts, master.WithLSSBoot(lss.FastscanOptions{Known: commonParts(expected)}))
			}
			if storeCfg {
				opts = append(opts, master.WithStoreAfterConfigure(store.All))
			}
			m, err := master.New(bus, opts...)
			if err != nil {
				return err
			}
			defer func() { _ = m.Close() }()
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			since := time.Now()
			if err := m.Start(); err != nil {
				return err
			}
			var bootErr error
			select {
			case bootErr = <-ev.done:
			case <-ctx.Done():
				if cmd.Context().Err() != nil {
					return cmd.Context().Err()
				}
				bootErr = ctx.Err()
			}
			// Optional slaves boot when their boot-up arrives: give them a
			// moment.
			grace := time.Now().Add(time.Second)
			for time.Now().Before(grace) && ctx.Err() == nil {
				pending := false
				for _, id := range ids {
					if _, done := m.SlaveStatus(id); !done {
						pending = true
					}
				}
				if !pending {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			r := bootResult{Master: canopen.NodeID(masterID), LSS: lssStep, Started: !noStart && bootErr == nil, Slaves: []bootSlave{},
				ElapsedMs: time.Since(since).Milliseconds()}
			if bootErr != nil {
				r.Error = bootErr.Error()
				if errors.Is(bootErr, context.DeadlineExceeded) {
					r.Error = fmt.Sprintf("the boot-up did not end within --max-time %v", timeout)
				}
			}
			ev.mu.Lock()
			defer ev.mu.Unlock()
			for _, id := range ids {
				sc := cfgs[id]
				s := bootSlave{Node: id, Mandatory: !opt[id], Config: sc.file, Entries: len(sc.entries)}
				if sc.expect != nil {
					aj := addressOf(*sc.expect)
					s.Expected = &aj
				}
				if a, ok := ev.lss[id]; ok {
					aj := addressOf(a)
					s.LSS = &aj
				}
				st, done := m.SlaveStatus(id)
				switch {
				case !done:
					s.Status = "not booted (no boot-up)"
				default:
					s.Booted, s.Status = st.OK(), st.String()
					if _, txt, ok := strings.Cut(s.Status, ": "); ok {
						s.Status = txt
					}
					if st != master.BootOK {
						s.Code = string(rune(st))
					}
				}
				if e := ev.failed[id]; e != nil && !s.Booted {
					s.Error = e.Error()
				}
				if s.Booted {
					r.Booted++
				} else {
					r.Failed++
				}
				r.Slaves = append(r.Slaves, s)
			}
			if err := a.emit("boot", r); err != nil {
				return err
			}
			if r.Failed > 0 || r.Error != "" {
				return errs.ErrFindings
			}
			return nil
		},
	}
	bf.add(c, true)
	bf.addFD(c, "boot a CANopen FD network (USDO from --master-id, FD NMT and heartbeats)")
	sf.add(c, false)
	c.Flags().StringArrayVar(&slaves, "slave", nil, "A slave: NODE, or NODE=FILE with its DCF or concise DCF (repeatable)")
	c.Flags().StringArrayVar(&expects, "expect", nil, "Expected identity of a slave: NODE=vendor:product:revision:serial in hexadecimal, 0 not checked (repeatable)")
	c.Flags().StringVar(&optional, "optional", "", "Slaves that are not mandatory: 5,7-9")
	c.Flags().IntVar(&masterID, "master-id", int(canopen.MaxNodeID), "Node-ID of canopenctl's manager node")
	c.Flags().BoolVar(&lssStep, "lss", false, "Give unconfigured devices the node-ID of the slave whose --expect identity they have (LSS Fastscan)")
	c.Flags().BoolVar(&noStart, "no-start", false, "Leave the slaves pre-operational")
	c.Flags().BoolVar(&storeCfg, "store", false, "Make each configured slave save its parameters (0x1010:01)")
	c.Flags().DurationVar(&bootTime, "boot-time", 10*time.Second, "How long the boot-up waits for the mandatory slaves (0x1F89; 0: no limit)")
	c.Flags().DurationVar(&timeout, "max-time", time.Minute, "Bound of the whole boot-up")
	c.Flags().DurationVar(&lssWindow, "lss-window", lss.DefaultFastscanTimeout, "Listening window of each Fastscan query of --lss")
	return c
}
