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
	"github.com/otfabric/go-canopen/lss"
	"github.com/spf13/cobra"
)

// lssFlags are the flags of the lss commands.
type lssFlags struct {
	bf      busFlags
	timeout time.Duration
	retries int
	window  time.Duration
}

func (l *lssFlags) add(c *cobra.Command) {
	l.bf.add(c, true)
	c.Flags().DurationVar(&l.timeout, "timeout", lss.DefaultTimeout, "Response timeout of a confirmed LSS service")
	c.Flags().IntVar(&l.retries, "retries", lss.DefaultRetries, "Re-sends of a request without response (0..10)")
	c.Flags().DurationVar(&l.window, "window", lss.DefaultFastscanTimeout, "Listening window of each Fastscan query")
}

func (l *lssFlags) validate() error {
	if l.timeout <= 0 || l.timeout > time.Minute {
		return errs.Invalid("--timeout %v: 1ms..1m", l.timeout)
	}
	if l.retries < 0 || l.retries > 10 {
		return errs.Invalid("--retries %d: 0..10", l.retries)
	}
	if l.window <= 0 || l.window > 10*time.Second {
		return errs.Invalid("--window %v: 1ms..10s", l.window)
	}
	return nil
}

// open checks the transmission and opens an LSS master; done closes it
// and the bus.
func (l *lssFlags) open(what string) (m *lss.Master, done func(), err error) {
	if err := l.validate(); err != nil {
		return nil, nil, err
	}
	if err := l.bf.check(what); err != nil {
		return nil, nil, err
	}
	bus, err := l.bf.open(false)
	if err != nil {
		return nil, nil, err
	}
	m = lss.NewMaster(bus, lss.WithTimeout(l.timeout), lss.WithRetries(l.retries), lss.WithFastscanTimeout(l.window))
	if err := m.Start(); err != nil {
		_ = bus.Close()
		return nil, nil, err
	}
	return m, func() { _ = m.Close(); _ = bus.Close() }, nil
}

// lssErr makes the timeout of a confirmed service a "no response".
func lssErr(err error) error {
	if errors.Is(err, canopen.ErrTimeout) {
		return errs.New(errs.KindTimeout, errs.CodeNoResponse, "no LSS slave answered: %w", err)
	}
	return err
}

// addressJSON is an LSS address.
type addressJSON struct {
	Address  string `json:"address"`
	Vendor   string `json:"vendorId"`
	Product  string `json:"productCode"`
	Revision string `json:"revisionNumber"`
	Serial   string `json:"serialNumber"`
}

func addressOf(a lss.Address) addressJSON {
	return addressJSON{Address: a.String(), Vendor: cli.Hex(uint64(a.Vendor), 8), Product: cli.Hex(uint64(a.Product), 8),
		Revision: cli.Hex(uint64(a.Revision), 8), Serial: cli.Hex(uint64(a.Serial), 8)}
}

// lssResult is the result of the lss commands that act on one slave or
// the network.
type lssResult struct {
	Action        string       `json:"action"`
	State         string       `json:"state,omitempty"`
	Address       *addressJSON `json:"address,omitempty"`
	NodeID        *int         `json:"nodeId,omitempty"`
	Configured    *bool        `json:"configured,omitempty"`
	BitRate       string       `json:"bitRate,omitempty"`
	Stored        bool         `json:"stored,omitempty"`
	DelayMs       *int64       `json:"delayMs,omitempty"`
	Found         *bool        `json:"found,omitempty"`
	NonConfigured bool         `json:"nonConfigured,omitempty"`
	text          string
}

func (r lssResult) Text() string { return r.text }

func newLSSCmd(a *app) *cobra.Command {
	c := &cobra.Command{
		Use:   "lss",
		Short: "Layer setting services (CiA 305): node-IDs and bit rates of devices, Fastscan",
		Long: `The lss commands are an LSS master (CiA 305, on 0x7E5/0x7E4). Devices are
addressed by their LSS address, the four numbers of their identity object
0x1018 written vendor:product:revision:serial in hexadecimal
(00000F7A:0000C0C7:00010000:00000005).

An LSS slave is in the waiting or the configuration state. switch puts every
slave in one state; select puts the slave with an address in the
configuration state. inquire, configure and activate act on the slave(s) in
the configuration state; given --address they select that slave first and
switch every slave back to waiting afterwards. fastscan lists the
unconfigured devices (node-ID 0xFF); assign-all gives each of them a
node-ID. A device that received a node-ID applies it when it leaves the
configuration state (reset communication) or at its next reset; store makes
it keep it across power cycles.

Every lss command transmits and changes the LSS state of devices: off virtual
interfaces each needs --yes. An unanswered confirmed service is exit 5, a
refusal by the device exit 12 (lss_refused).`,
	}
	c.AddCommand(newLSSSwitchCmd(a), newLSSSelectCmd(a), newLSSInquireCmd(a), newLSSConfigureCmd(a), newLSSActivateCmd(a),
		newLSSIdentifyCmd(a), newLSSFastscanCmd(a), newLSSAssignCmd(a))
	return c
}

func newLSSSwitchCmd(a *app) *cobra.Command {
	var lf lssFlags
	c := &cobra.Command{
		Use:   "switch <waiting|configuration>",
		Short: "Switch every LSS slave to the waiting or the configuration state (switch state global)",
		Long: `switch sends "switch state global" (unconfirmed): every LSS slave enters the
waiting or the configuration state. A device whose node-ID was configured
while it had none applies it when it is switched to waiting.`,
		Example: `  canopenctl lss switch configuration -i vcan0
  canopenctl lss switch waiting -i can0 --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var st lss.State
			switch strings.ToLower(args[0]) {
			case "waiting", "wait":
				st = lss.Waiting
			case "configuration", "config":
				st = lss.Configuration
			default:
				return errs.Invalid("unknown LSS state %q (waiting or configuration)", args[0])
			}
			m, done, err := lf.open("lss switch")
			if err != nil {
				return err
			}
			defer done()
			if err := m.SwitchStateGlobal(cmd.Context(), st); err != nil {
				return err
			}
			return a.emit("lss switch", lssResult{Action: "switch", State: st.String(), text: "every LSS slave switched to " + st.String()})
		},
	}
	lf.add(c)
	return c
}

func newLSSSelectCmd(a *app) *cobra.Command {
	var lf lssFlags
	c := &cobra.Command{
		Use:   "select <address>",
		Short: "Switch the slave with an LSS address to the configuration state (switch state selective)",
		Long: `select sends "switch state selective" with an LSS address: the slave that has
it enters the configuration state and confirms. No answer is exit 5. Slaves
already in the configuration state stay there; "lss switch waiting" ends
the configuration.`,
		Example: `  canopenctl lss select 00000F7A:0000C0C7:00010000:00000005 -i vcan0`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			addr, err := cli.ParseLSSAddress(args[0])
			if err != nil {
				return err
			}
			m, done, err := lf.open("lss select")
			if err != nil {
				return err
			}
			defer done()
			if err := m.SwitchStateSelective(cmd.Context(), addr); err != nil {
				return lssErr(err)
			}
			aj := addressOf(addr)
			return a.emit("lss select", lssResult{Action: "select", State: lss.Configuration.String(), Address: &aj,
				text: "slave " + addr.String() + " in the configuration state"})
		},
	}
	lf.add(c)
	return c
}

// withSlave runs fn on the slave in the configuration state, or, with an
// address, selects that slave around fn.
func withSlave(ctx context.Context, m *lss.Master, address string, fn func(ctx context.Context) error) error {
	if address == "" {
		return lssErr(fn(ctx))
	}
	addr, err := cli.ParseLSSAddress(address)
	if err != nil {
		return err
	}
	return lssErr(m.Configure(ctx, addr, fn))
}

func newLSSInquireCmd(a *app) *cobra.Command {
	var (
		lf      lssFlags
		address string
	)
	c := &cobra.Command{
		Use:   "inquire",
		Short: "Read the LSS address and node-ID of the slave in the configuration state",
		Long: `inquire reads the four parts of the LSS address (inquire identity) and the
active node-ID (inquire node-ID; 255 when the device has none) of the slave
in the configuration state. Exactly one slave should be in it: select it
first, or give --address. No answer is exit 5.`,
		Example: `  canopenctl lss select 00000F7A:0000C0C7:00010000:00000005 -i vcan0 && canopenctl lss inquire -i vcan0
  canopenctl lss inquire --address 00000F7A:0000C0C7:00010000:00000005 -i vcan0`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if address != "" {
				if _, err := cli.ParseLSSAddress(address); err != nil {
					return err
				}
			}
			m, done, err := lf.open("lss inquire")
			if err != nil {
				return err
			}
			defer done()
			var addr lss.Address
			var id canopen.NodeID
			err = withSlave(cmd.Context(), m, address, func(ctx context.Context) error {
				var err error
				if addr, err = m.InquireAddress(ctx); err != nil {
					return err
				}
				id, err = m.InquireNodeID(ctx)
				return err
			})
			if err != nil {
				return err
			}
			aj, n, conf := addressOf(addr), int(id), id != lss.Unconfigured
			text := fmt.Sprintf("slave %s: node-ID %d", addr, id)
			if !conf {
				text = fmt.Sprintf("slave %s: unconfigured (no node-ID)", addr)
			}
			return a.emit("lss inquire", lssResult{Action: "inquire", Address: &aj, NodeID: &n, Configured: &conf, text: text})
		},
	}
	lf.add(c)
	c.Flags().StringVar(&address, "address", "", "Select the slave with this LSS address first (vendor:product:revision:serial)")
	return c
}

// parseBitRate parses --bit-rate: kbit/s of the CiA 301 table, or auto.
func parseBitRate(s string) (lss.BitTiming, error) {
	if strings.EqualFold(s, "auto") {
		return lss.BitTimingAuto, nil
	}
	k, err := strconv.Atoi(strings.TrimSuffix(strings.ToLower(s), "k"))
	if b, ok := lss.BitTimingFor(k); err == nil && k > 0 && ok {
		return b, nil
	}
	return 0, errs.Invalid("invalid --bit-rate %q (1000, 800, 500, 250, 125, 50, 20, 10 kbit/s or auto)", s)
}

func newLSSConfigureCmd(a *app) *cobra.Command {
	var (
		lf      lssFlags
		address string
		nodeID  string
		bitRate string
		store   bool
	)
	c := &cobra.Command{
		Use:   "configure",
		Short: "Configure the node-ID and bit rate of the slave in the configuration state, and store them",
		Long: `configure sends, to the slave in the configuration state (or, with --address,
to that slave, selected for it), in this order: configure node-ID
(--node-id 1..127, or "none" to remove it), configure bit timing (--bit-rate
in kbit/s from the CiA 301 table, or auto; it takes effect with "lss
activate") and store configuration (--store: the device keeps both across
power cycles). At least one is needed. A refusal is exit 12 with the
device's error code.

The new node-ID is pending: a device that had none applies it when it
leaves the configuration state ("lss switch waiting", or at once with
--address), a configured device at its next NMT reset.`,
		Example: `  canopenctl lss configure --address 00000F7A:0000C0C7:00010000:00001001 --node-id 5 --store -i vcan0
  canopenctl lss configure --bit-rate 250 -i can0 --yes`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if nodeID == "" && bitRate == "" && !store {
				return errs.Invalid("give --node-id, --bit-rate or --store")
			}
			id := canopen.NodeID(0)
			if nodeID != "" {
				if strings.EqualFold(nodeID, "none") || nodeID == "255" || strings.EqualFold(nodeID, "0xff") {
					id = lss.Unconfigured
				} else {
					var err error
					if id, err = cli.ParseNode(nodeID, false); err != nil {
						return err
					}
				}
			}
			var bt lss.BitTiming
			if bitRate != "" {
				var err error
				if bt, err = parseBitRate(bitRate); err != nil {
					return err
				}
			}
			if address != "" {
				if _, err := cli.ParseLSSAddress(address); err != nil {
					return err
				}
			}
			m, done, err := lf.open("lss configure")
			if err != nil {
				return err
			}
			defer done()
			r := lssResult{Action: "configure"}
			var parts []string
			err = withSlave(cmd.Context(), m, address, func(ctx context.Context) error {
				if id != 0 {
					if err := m.SetNodeID(ctx, id); err != nil {
						return err
					}
					n := int(id)
					r.NodeID = &n
					parts = append(parts, fmt.Sprintf("node-ID %d", id))
					if id == lss.Unconfigured {
						parts[len(parts)-1] = "node-ID removed"
					}
				}
				if bitRate != "" {
					if err := m.SetBitTiming(ctx, bt); err != nil {
						return err
					}
					r.BitRate = bt.String()
					parts = append(parts, "bit rate "+bt.String())
				}
				if store {
					if err := m.Store(ctx); err != nil {
						return err
					}
					r.Stored = true
					parts = append(parts, "stored")
				}
				return nil
			})
			if err != nil {
				return err
			}
			who := "slave in the configuration state"
			if address != "" {
				addr, _ := cli.ParseLSSAddress(address)
				aj := addressOf(addr)
				r.Address = &aj
				who = "slave " + addr.String()
			}
			r.text = who + ": " + strings.Join(parts, ", ")
			return a.emit("lss configure", r)
		},
	}
	lf.add(c)
	c.Flags().StringVar(&address, "address", "", "Select the slave with this LSS address for the configuration (vendor:product:revision:serial)")
	c.Flags().StringVar(&nodeID, "node-id", "", "Node-ID to configure: 1..127, or none")
	c.Flags().StringVar(&bitRate, "bit-rate", "", "Bit rate to configure in kbit/s (1000, 800, 500, 250, 125, 50, 20, 10) or auto")
	c.Flags().BoolVar(&store, "store", false, "Make the slave store its node-ID and bit rate")
	return c
}

func newLSSActivateCmd(a *app) *cobra.Command {
	var (
		lf    lssFlags
		delay time.Duration
	)
	c := &cobra.Command{
		Use:   "activate",
		Short: "Make the slaves in the configuration state switch to their configured bit rate",
		Long: `activate sends "activate bit timing parameters" (unconfirmed): every slave in
the configuration state switches to the bit rate configured with "lss
configure --bit-rate" after --delay, and stays silent for another --delay.
canopenctl's own interface keeps its bit rate: change it yourself (ip link,
--slcan-bitrate) within the delay.`,
		Example: `  canopenctl lss activate --delay 500ms -i can0 --yes`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if delay < 0 || delay > 65535*time.Millisecond {
				return errs.Invalid("--delay %v: 0..65535ms", delay)
			}
			m, done, err := lf.open("lss activate")
			if err != nil {
				return err
			}
			defer done()
			if err := m.ActivateBitTiming(cmd.Context(), delay); err != nil {
				return err
			}
			ms := delay.Milliseconds()
			return a.emit("lss activate", lssResult{Action: "activate", DelayMs: &ms,
				text: fmt.Sprintf("activate bit timing sent (switch delay %d ms)", ms)})
		},
	}
	lf.add(c)
	c.Flags().DurationVar(&delay, "delay", 100*time.Millisecond, "Switch delay (0..65535ms)")
	return c
}

func newLSSIdentifyCmd(a *app) *cobra.Command {
	var (
		lf      lssFlags
		address string
		to      string
	)
	c := &cobra.Command{
		Use:   "identify",
		Short: "Ask whether a device without node-ID (or with an LSS address) is on the network",
		Long: `identify sends "identify non-configured remote slave": devices with neither an
active nor a pending node-ID answer. With --address it sends "identify remote
slave" instead: devices whose LSS address is --address (or in the range
--address..--to: vendor-ID and product code equal, revision and serial
number inclusive ranges) answer. The result says whether any device
answered; silence is not an error.`,
		Example: `  canopenctl lss identify -i vcan0
  canopenctl lss identify --address 00000F7A:0000C0C7:00010000:00000000 --to 00000F7A:0000C0C7:00010000:FFFFFFFF -i vcan0`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var lo, hi lss.Address
			if address != "" {
				var err error
				if lo, err = cli.ParseLSSAddress(address); err != nil {
					return err
				}
				hi = lo
				if to != "" {
					if hi, err = cli.ParseLSSAddress(to); err != nil {
						return err
					}
				}
				if lo.Vendor != hi.Vendor || lo.Product != hi.Product || lo.Revision > hi.Revision || lo.Serial > hi.Serial {
					return errs.Invalid("invalid range %v..%v: vendor and product equal, revision and serial ascending", lo, hi)
				}
			} else if to != "" {
				return errs.Invalid("--to needs --address")
			}
			m, done, err := lf.open("lss identify")
			if err != nil {
				return err
			}
			defer done()
			var found bool
			r := lssResult{Action: "identify"}
			if address == "" {
				found, err = m.IdentifyNonConfigured(cmd.Context())
				r.NonConfigured = true
			} else {
				found, err = m.Identify(cmd.Context(), lo, hi)
				aj := addressOf(lo)
				r.Address = &aj
			}
			if err != nil {
				return err
			}
			r.Found = &found
			switch {
			case address == "" && found:
				r.text = "a device without node-ID answered"
			case address == "":
				r.text = "no device without node-ID answered"
			case found:
				r.text = "a device in " + lo.String() + ".." + hi.String() + " answered"
			default:
				r.text = "no device in " + lo.String() + ".." + hi.String() + " answered"
			}
			return a.emit("lss identify", r)
		},
	}
	lf.add(c)
	c.Flags().StringVar(&address, "address", "", "Identify the devices with this LSS address (the low end with --to)")
	c.Flags().StringVar(&to, "to", "", "High end of the LSS address range of --address")
	return c
}

// knownFlags are the known parts of the address of a Fastscan.
type knownFlags struct{ vendor, product, revision string }

func (k *knownFlags) add(c *cobra.Command) {
	c.Flags().StringVar(&k.vendor, "vendor", "", "Only devices with this vendor-ID (verified instead of scanned: faster)")
	c.Flags().StringVar(&k.product, "product", "", "Only devices with this product code")
	c.Flags().StringVar(&k.revision, "revision", "", "Only devices with this revision number")
}

func (k *knownFlags) parse() (map[lss.Part]uint32, error) {
	out := map[lss.Part]uint32{}
	for _, x := range []struct {
		p lss.Part
		s string
	}{{lss.PartVendor, k.vendor}, {lss.PartProduct, k.product}, {lss.PartRevision, k.revision}} {
		if x.s == "" {
			continue
		}
		v, err := cli.ParseUint32(x.s)
		if err != nil {
			return nil, err
		}
		out[x.p] = v
	}
	return out, nil
}

type fastscanResult struct {
	Devices []addressJSON `json:"devices"`
}

func (r fastscanResult) Text() string {
	var b strings.Builder
	for _, d := range r.Devices {
		fmt.Fprintf(&b, "%s  vendor %s  product %s  revision %s  serial %s\n", d.Address, d.Vendor, d.Product, d.Revision, d.Serial)
	}
	fmt.Fprintf(&b, "%d unconfigured devices", len(r.Devices))
	return b.String()
}

func (r fastscanResult) Headers() []string {
	return []string{"ADDRESS", "VENDOR", "PRODUCT", "REVISION", "SERIAL"}
}

func (r fastscanResult) Rows() [][]string {
	var rows [][]string
	for _, d := range r.Devices {
		rows = append(rows, []string{d.Address, d.Vendor, d.Product, d.Revision, d.Serial})
	}
	return rows
}

func newLSSFastscanCmd(a *app) *cobra.Command {
	var (
		lf    lssFlags
		known knownFlags
		limit int
	)
	c := &cobra.Command{
		Use:   "fastscan",
		Short: "List the LSS addresses of the unconfigured devices (Fastscan)",
		Long: `fastscan finds the LSS addresses of the devices without node-ID with LSS
Fastscan, bit by bit, without selecting any. Every query costs a listening
window (--window): a full scan of one device takes about 130 windows, fewer
with known parts (--vendor, --product, --revision, which are then verified
instead of scanned). Configured devices do not take part: "lss inquire" or
"scan" find those.`,
		Example: `  canopenctl lss fastscan -i vcan0
  canopenctl lss fastscan --vendor 0xF7A --product 0xC0C7 --window 10ms -i vcan0 --format table`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			k, err := known.parse()
			if err != nil {
				return err
			}
			if limit < 0 {
				return errs.Invalid("--max %d: 0 or more", limit)
			}
			m, done, err := lf.open("lss fastscan")
			if err != nil {
				return err
			}
			defer done()
			found, err := m.Fastscan(cmd.Context(), lss.FastscanOptions{Known: k, Max: limit})
			if err != nil {
				return err
			}
			r := fastscanResult{Devices: []addressJSON{}}
			for _, d := range found {
				r.Devices = append(r.Devices, addressOf(d))
			}
			return a.emit("lss fastscan", r)
		},
	}
	lf.add(c)
	known.add(c)
	c.Flags().IntVar(&limit, "max", 0, "Stop after this many devices (0: all)")
	return c
}

type assignJSON struct {
	addressJSON
	NodeID canopen.NodeID `json:"nodeId"`
}

type assignResult struct {
	First       canopen.NodeID `json:"first"`
	Stored      bool           `json:"stored"`
	Assignments []assignJSON   `json:"assignments"`
	Error       string         `json:"error,omitempty"`
}

func (r assignResult) Text() string {
	var b strings.Builder
	for _, x := range r.Assignments {
		fmt.Fprintf(&b, "node-ID %3d -> %s\n", x.NodeID, x.Address)
	}
	if r.Error != "" {
		fmt.Fprintf(&b, "error: %s\n", r.Error)
	}
	fmt.Fprintf(&b, "%d devices given a node-ID", len(r.Assignments))
	if r.Stored {
		b.WriteString(" (stored)")
	}
	return b.String()
}

func (r assignResult) Headers() []string { return []string{"NODE", "ADDRESS"} }
func (r assignResult) Rows() [][]string {
	var rows [][]string
	for _, x := range r.Assignments {
		rows = append(rows, []string{strconv.Itoa(int(x.NodeID)), x.Address})
	}
	return rows
}

func newLSSAssignCmd(a *app) *cobra.Command {
	var (
		lf    lssFlags
		known knownFlags
		first int
		avoid string
		limit int
		store bool
	)
	c := &cobra.Command{
		Use:   "assign-all",
		Short: "Give every unconfigured device a node-ID (Fastscan, configure node-ID)",
		Long: `assign-all gives the devices without node-ID node-IDs from --first upwards,
skipping --avoid: one by one, it selects a device with Fastscan, checks that
it alone is in the configuration state, configures its node-ID (and makes it
store it with --store) and switches it back to waiting, upon which the
device applies the node-ID (reset communication) and boots. It ends when no
unconfigured device answers. A node-ID once requested is never offered
again, even when the device did not confirm.

Node-IDs already used by configured devices are not detected: give them
with --avoid (scan finds them). When a step fails after some devices were
configured, the result lists them with the error and the exit code is 1.`,
		Example: `  canopenctl lss assign-all --first 10 -i vcan0
  canopenctl lss assign-all --first 2 --avoid 1-9 --store --vendor 0xF7A -i can0 --yes`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !canopen.NodeID(first).Valid() {
				return errs.Invalid("--first %d: 1..127", first)
			}
			if limit < 0 {
				return errs.Invalid("--max %d: 0 or more", limit)
			}
			var av []canopen.NodeID
			if avoid != "" {
				var err error
				if av, err = cli.ParseNodeList(avoid); err != nil {
					return err
				}
			}
			k, err := known.parse()
			if err != nil {
				return err
			}
			m, done, err := lf.open("lss assign-all")
			if err != nil {
				return err
			}
			defer done()
			opts := []lss.AssignOption{lss.AssignFastscan(lss.FastscanOptions{Known: k}), lss.AssignMax(limit), lss.AssignAvoid(av...)}
			if store {
				opts = append(opts, lss.AssignStore())
			}
			got, err := m.AssignAll(cmd.Context(), canopen.NodeID(first), opts...)
			if err != nil && len(got) == 0 {
				return lssErr(err)
			}
			r := assignResult{First: canopen.NodeID(first), Stored: store, Assignments: []assignJSON{}}
			for _, x := range got {
				r.Assignments = append(r.Assignments, assignJSON{addressJSON: addressOf(x.Address), NodeID: x.NodeID})
			}
			if err != nil {
				r.Error = err.Error()
			}
			if err := a.emit("lss assign-all", r); err != nil {
				return err
			}
			if r.Error != "" {
				return errs.ErrFindings
			}
			return nil
		},
	}
	lf.add(c)
	known.add(c)
	c.Flags().IntVar(&first, "first", 0, "First node-ID to give (required)")
	c.Flags().StringVar(&avoid, "avoid", "", "Node-IDs not to give (in use): 1-9,20")
	c.Flags().IntVar(&limit, "max", 0, "Stop after this many devices (0: all)")
	c.Flags().BoolVar(&store, "store", false, "Make every device store its node-ID")
	_ = c.MarkFlagRequired("first")
	return c
}
