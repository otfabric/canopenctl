// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/otfabric/canopenctl/internal/cli"
	"github.com/otfabric/canopenctl/internal/errs"
	"github.com/otfabric/go-canbus"
	"github.com/otfabric/go-canbus/slcan"
	"github.com/otfabric/go-canbus/socketcan"
	"github.com/otfabric/go-canbus/virtual"
	"github.com/otfabric/go-canopen"
	"github.com/otfabric/go-canopen/master"
	"github.com/otfabric/go-canopen/sdo"
	"github.com/otfabric/go-canopen/usdo"
	"github.com/spf13/cobra"
)

// openOpts are what a command asks of a transport.
type openOpts struct {
	passive bool // the command never transmits: slcan adapters open listen-only
	fd      bool // open the interface for CAN FD frames (--fd)
}

// slcanFlags are the --slcan-* flags of an invocation.
type slcanFlags struct{ bitrate, baud int }

// slcanSettings are the flags of the running invocation.
var (
	slcanMu       sync.Mutex
	slcanSettings = slcanFlags{bitrate: 500000, baud: 115200}
)

func setSlcan(f slcanFlags) {
	slcanMu.Lock()
	slcanSettings = f
	slcanMu.Unlock()
}

func slcanConfig() slcanFlags {
	slcanMu.Lock()
	defer slcanMu.Unlock()
	return slcanSettings
}

// openSlcan opens "slcan:<device>" (a serial adapter) or
// "slcan:tcp://host:port" (an adapter behind a serial server).
func openSlcan(iface string, o openOpts) (canbus.Transport, error) {
	spec := strings.TrimPrefix(iface, cli.SlcanPrefix)
	set := slcanConfig()
	cfg := slcan.Config{Bitrate: set.bitrate, ListenOnly: o.passive, Name: iface}
	var c *slcan.Conn
	var err error
	if addr, ok := strings.CutPrefix(spec, "tcp://"); ok {
		var nc net.Conn
		if nc, err = net.DialTimeout("tcp", addr, 5*time.Second); err == nil {
			c, err = slcan.New(nc, cfg)
		}
	} else {
		c, err = slcan.Open(spec, set.baud, cfg)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", canbus.ErrInterfaceDown, iface, err)
	}
	return c, nil
}

// The in-process virtual bus ("--interface virtual"), shared by the ports
// of one process: a command and the nodes a test (or simulate) runs.
var (
	virtualMu  sync.Mutex
	virtualBus *virtual.Bus
)

func sharedVirtual() *virtual.Bus {
	virtualMu.Lock()
	defer virtualMu.Unlock()
	if virtualBus == nil {
		virtualBus = virtual.New(virtual.Config{Name: cli.Virtual})
	}
	return virtualBus
}

// openTransport opens an interface; tests may replace it.
var openTransport = func(iface string, o openOpts) (canbus.Transport, error) {
	if strings.HasPrefix(iface, cli.SlcanPrefix) {
		return openSlcan(iface, o)
	}
	if iface == cli.Virtual {
		if o.fd {
			return sharedVirtual().Open(virtual.WithQueueSize(65536), virtual.WithFD())
		}
		return sharedVirtual().Open(virtual.WithQueueSize(65536))
	}
	if o.fd {
		return socketcan.Open(iface, socketcan.WithFD())
	}
	return socketcan.Open(iface)
}

// openBus opens an interface wrapped in a Bus that reads from its first
// subscription on: no frame between opening and subscribing is lost.
func openBus(iface string, o openOpts) (*canbus.Bus, error) {
	if iface == "" {
		return nil, errs.New(errs.KindUsage, errs.CodeInvalidInput, "no interface: give --interface (-i) or set CANOPENCTL_INTERFACE")
	}
	t, err := openTransport(iface, o)
	if err != nil {
		return nil, err
	}
	b, err := canbus.NewBus(t, canbus.WithInterfaceName(iface), canbus.WithStartOnSubscribe())
	if err != nil {
		_ = t.Close()
		return nil, err
	}
	return b, nil
}

// busFlags are the interface flags of a command on the network.
type busFlags struct {
	iface string
	yes   bool
	fd    bool // --fd: CANopen FD (CiA 1301)
	brs   bool // --brs: CAN FD bit rate switching
}

// addFD adds --fd and --brs to a command that speaks CANopen FD.
func (b *busFlags) addFD(c *cobra.Command, what string) {
	c.Flags().BoolVar(&b.fd, "fd", false, "CANopen FD (CiA 1301): open the interface for CAN FD and "+what)
	c.Flags().BoolVar(&b.brs, "brs", false, "With --fd: send CAN FD frames with bit rate switching")
}

// open opens the interface of the command.
func (b *busFlags) open(passive bool) (*canbus.Bus, error) {
	return openBus(b.iface, openOpts{passive: passive, fd: b.fd})
}

// checkFD rejects --brs without --fd.
func (b *busFlags) checkFD() error {
	if b.brs && !b.fd {
		return errs.Invalid("--brs needs --fd")
	}
	return nil
}

// masterOptions returns the master options of the flags: CANopen FD with
// --fd.
func (b *busFlags) masterOptions() []master.Option {
	if b.fd {
		return []master.Option{master.WithFD(b.brs)}
	}
	return nil
}

func (b *busFlags) add(c *cobra.Command, transmits bool) {
	c.Flags().StringVarP(&b.iface, "interface", "i", os.Getenv("CANOPENCTL_INTERFACE"),
		"CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)")
	if transmits {
		c.Flags().BoolVar(&b.yes, "yes", false, "Confirm a state-changing transmission on an interface that is not virtual")
	}
}

// check runs the transmit safety check of a state-changing command.
func (b *busFlags) check(what string) error {
	if b.iface == "" {
		return errs.New(errs.KindUsage, errs.CodeInvalidInput, "no interface: give --interface (-i) or set CANOPENCTL_INTERFACE")
	}
	return cli.TransmitCheck(b.iface, b.yes, what)
}

// sdoFlags are the SDO client flags of a command.
type sdoFlags struct {
	timeout   time.Duration
	retries   int
	block     bool
	blockSize int
	clientID  int // USDO client node-ID (--fd)
}

func (s *sdoFlags) add(c *cobra.Command, block bool) {
	c.Flags().DurationVar(&s.timeout, "timeout", sdo.DefaultTimeout, "SDO response timeout")
	c.Flags().IntVar(&s.retries, "retries", 0, "Repeat a transfer that timed out up to this many times (0..10)")
	if block {
		c.Flags().BoolVar(&s.block, "block", false, "Use SDO block transfer")
		c.Flags().IntVar(&s.blockSize, "block-size", sdo.MaxBlockSize, "Segments per block the client asks for in block uploads (1..127)")
	}
}

// addFD adds --client-id, the node-ID of the USDO client of --fd.
func (s *sdoFlags) addFD(c *cobra.Command) {
	c.Flags().IntVar(&s.clientID, "client-id", int(canopen.MaxNodeID), "With --fd: node-ID of canopenctl's USDO client (its requests go out on 0x600 + it)")
}

func (s *sdoFlags) validate() error {
	if s.timeout <= 0 || s.timeout > time.Minute {
		return errs.Invalid("--timeout %v: 1ms..1m", s.timeout)
	}
	if s.retries < 0 || s.retries > 10 {
		return errs.Invalid("--retries %d: 0..10", s.retries)
	}
	if s.blockSize != 0 && (s.blockSize < 1 || s.blockSize > sdo.MaxBlockSize) {
		return errs.Invalid("--block-size %d: 1..127", s.blockSize)
	}
	if s.clientID != 0 && !canopen.NodeID(s.clientID).Valid() {
		return errs.Invalid("--client-id %d: 1..127", s.clientID)
	}
	return nil
}

// usdoOptions returns the USDO client options of the flags.
func (s *sdoFlags) usdoOptions(brs bool) []usdo.Option {
	return []usdo.Option{usdo.WithTimeout(s.timeout), usdo.WithBRS(brs)}
}

// masterOptions returns the master options of the SDO and bus flags: the
// SDO client options and, with --fd, CANopen FD with a USDO client.
func (s *sdoFlags) masterOptions(b *busFlags) []master.Option {
	opts := []master.Option{master.WithSDOOptions(s.options()...)}
	if b.fd {
		id := canopen.NodeID(s.clientID)
		if !id.Valid() {
			id = canopen.MaxNodeID
		}
		opts = append(opts, master.WithFD(b.brs), master.WithUSDOClient(id, s.usdoOptions(b.brs)...))
	}
	return opts
}

// remote reads and writes one node's dictionary: over SDO, or over USDO
// with --fd.
type remote interface {
	Upload(ctx context.Context, index uint16, sub uint8) ([]byte, error)
	Download(ctx context.Context, index uint16, sub uint8, data []byte) error
	// blocked reports whether a block (SDO) or bulk (USDO) transfer was
	// used.
	blocked() bool
	Close() error
}

type sdoRemote struct{ c *sdo.Client }

func (r sdoRemote) Upload(ctx context.Context, index uint16, sub uint8) ([]byte, error) {
	return r.c.Upload(ctx, index, sub)
}

func (r sdoRemote) Download(ctx context.Context, index uint16, sub uint8, data []byte) error {
	return r.c.Download(ctx, index, sub, data)
}

func (r sdoRemote) blocked() bool { return r.c.Stats().BlockTransfers > 0 }
func (r sdoRemote) Close() error  { return r.c.Close() }

type usdoRemote struct {
	c       *usdo.Client
	server  canopen.NodeID
	bulk    bool
	retries int
}

func (r usdoRemote) retry(ctx context.Context, f func() error) error {
	var err error
	for i := 0; i <= r.retries; i++ {
		if err = f(); err == nil || !errors.Is(err, canopen.ErrTimeout) || ctx.Err() != nil {
			return err
		}
	}
	return err
}

func (r usdoRemote) Upload(ctx context.Context, index uint16, sub uint8) ([]byte, error) {
	var b []byte
	err := r.retry(ctx, func() error {
		var err error
		b, err = r.c.Upload(ctx, r.server, index, sub)
		return err
	})
	return b, err
}

func (r usdoRemote) Download(ctx context.Context, index uint16, sub uint8, data []byte) error {
	return r.retry(ctx, func() error { return r.c.Download(ctx, r.server, index, sub, data, usdo.UseBulk(r.bulk)) })
}

func (r usdoRemote) blocked() bool { return r.c.Stats().Bulk > 0 }
func (r usdoRemote) Close() error  { return r.c.Close() }

// newRemote opens the dictionary client of a node on an open bus: an SDO
// client, or with --fd a USDO client (--block selects bulk downloads).
func newRemote(bus canopen.Bus, node canopen.NodeID, s *sdoFlags, b *busFlags) (remote, error) {
	if !b.fd {
		c, err := sdo.NewClient(bus, node, s.options()...)
		if err != nil {
			return nil, err
		}
		return sdoRemote{c}, nil
	}
	id := canopen.NodeID(s.clientID)
	if !id.Valid() {
		id = canopen.MaxNodeID
	}
	c, err := usdo.NewClient(bus, id, s.usdoOptions(b.brs)...)
	if err != nil {
		return nil, err
	}
	if err := c.Start(); err != nil {
		return nil, err
	}
	return usdoRemote{c: c, server: node, bulk: s.block, retries: s.retries}, nil
}

func (s *sdoFlags) options() []sdo.ClientOption {
	opts := []sdo.ClientOption{sdo.WithTimeout(s.timeout), sdo.WithRetries(s.retries)}
	if s.block {
		opts = append(opts, sdo.WithBlockMode(sdo.BlockAlways))
	}
	if s.blockSize > 0 {
		opts = append(opts, sdo.WithBlockSize(s.blockSize))
	}
	return opts
}

// bounded returns ctx limited to d when d > 0.
func bounded(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d > 0 {
		return context.WithTimeout(ctx, d)
	}
	return context.WithCancel(ctx)
}

// ended reports an error that ends a stream normally: the duration
// elapsed or the user interrupted.
func ended(ctx context.Context, err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return ctx.Err() != nil && errors.Is(err, context.Canceled)
}
