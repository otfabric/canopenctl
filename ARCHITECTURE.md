# Architecture

canopenctl is a thin command layer over go-canopen (CANopen) and go-canbus
(transports, traces). It owns parsing, output, exit codes and safety; every
CANopen operation is a library call.

```
main.go                 build metadata (ldflags), cmd.Execute
cmd/                    one file per command group (cobra)
  root.go               Run(ctx, args, stdin, stdout, stderr) → exit code; --format; version; completion
  transport.go          -i: socketcan.Open, slcan (serial or TCP), a port of the in-process virtual bus;
                        shared --interface/--yes/--fd and SDO client flags; remote (SDO, or USDO with --fd)
  monitor.go            monitor, decode                    → dissect.Tracker, capture readers
  sdo.go                sdo read/write/dump                → sdo.Client, eds (names and types)
  nmt.go                nmt, heartbeat watch, scan          → master (NMT, WaitState/WaitBootUp, Scan), nmt.HeartbeatConsumer
  emcy.go               emcy watch, sync, time              → emcy.Consumer, syncobj.Producer, timestamp.Producer
  pdo.go                pdo show/map/watch                  → pdo.ReadConfig, master.ConfigurePDO, pdo.Listener/Decoder
  eds.go                eds validate/show/convert/generate, dcf apply/export
                                                            → eds, codegen, master.DCFEntries/Configure, eds.FromDictionary
  cia402.go             cia402 *                            → cia402.Controller over the node's SDO client
  lss.go                lss *                               → lss.Master (switch, select, inquire, configure, Fastscan, AssignAll)
  mpdo.go               mpdo configure/scanner/dispatcher/watch
                                                            → master.ConfigureMPDO/WriteScannerList/WriteDispatcherList, mpdo.ParseMessage
  cia302.go             boot, cdcf *, store, restore, program *
                                                            → master (WithDictionary + AddManagerObjects, WithLSSBoot), cdcf,
                                                              master.ConfigureConcise/StoreParameters/RestoreDefaults/DownloadProgram
  simulate.go           simulate                            → node.New (+ WithDrive, WithLSS, WithFD, WithStore, WithProgram)
internal/cli            node-IDs, indices, data types, values, PDO mappings, EDS and trace loading, transmit safety
internal/output         text, table, JSON envelope, JSON Lines streams
internal/errs           kinds, stable codes, exit codes, classification of go-canbus/go-canopen errors
schemas/                JSON Schemas of the result, record and error documents
e2e/                    tests of the built binary as separate processes (simulated nodes on vcan0)
interop/                tests of the binary against the canopen-interop images (build tag interop)
```

**Run, not os.Exit.** `cmd.Run` takes its streams and returns the exit code,
so tests run the real command tree in process (`cmd/*_test.go`), including
usage errors.

**The in-process virtual bus.** `--interface virtual` gives every command a
port of one go-canbus `virtual.Bus` per process. The command tests start
go-canopen nodes on other ports of the same bus and run the commands against
them; `simulate -i virtual` runs a node no other process can reach.

**Safety** lives in one function, `cli.TransmitCheck`: the in-process bus,
vcan and vxcan (kind from rtnetlink) pass; anything else needs `--yes`.
Every state-changing command calls it after validating its input and before
opening the interface. Passive commands open slcan adapters listen-only.

**Errors.** Commands return typed `errs.Error`s or library errors;
`errs.Classify` maps the sentinels: a remote `*canopen.AbortError` becomes
kind `abort` (exit 11) with the abort code, object and CiA 301 text in the
error document (a `*usdo.AbortError` likewise, with its SDO equivalent and
`usdoCode`); `canopen.ErrTimeout` timeout; LSS refusals, conflicts, failed
program downloads and boot slave processes kind `device`; `canopen.ErrProtocol` protocol;
`*cia402.Error` kind `device` with the drive's state; go-canbus sentinels as
in canbusctl. `errs.ErrFindings` ends commands whose result reports problems
(exit 1).

**Output.** Results implement `Text()` (and `Headers()`/`Rows()` for
tables); the JSON form is the struct itself inside the versioned envelope.
Streams write a line of text or a JSON Lines record per event and a summary.

**Concurrency.** go-canopen services deliver events on their own goroutines
(heartbeat consumer, EMCY consumer, PDO listener, node state handler);
commands forward them through buffered channels to the command goroutine,
which alone writes output.
