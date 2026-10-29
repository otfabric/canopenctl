# canopenctl command reference

Generated from `canopenctl <command> --help` by scripts/gen-cli-doc.sh; do not edit.
Exit codes: [exitcodes.md](exitcodes.md). JSON output: [json-output.md](json-output.md).

```text
canopenctl is a command-line toolkit for CANopen (CiA 301, CiA 302 network
management, CiA 305 LSS, CiA 306 EDS/DCF, CiA 402 drives, CiA 1301 CANopen
FD), built on github.com/otfabric/go-canopen and go-canbus.

Read-only by default: monitor, decode, heartbeat watch, emcy watch, pdo
watch and mpdo watch never transmit; sdo read, sdo dump, scan, pdo show, dcf
export, program status and cia402 state only send SDO read requests.
Commands that change a device or the network (sdo write, nmt, pdo map, dcf
apply, cdcf apply, cia402 enable/disable/fault-reset/quick-stop/mode/move/
velocity, sync, time, lss, mpdo configure/scanner/dispatcher, boot, store,
restore, program download/control, simulate) need --yes on any interface
that is not virtual (vcan, vxcan or the in-process bus "virtual"), and are
bounded (--count, --duration, --timeout). --fd selects CANopen FD (USDO
instead of SDO, CAN FD frames) where a command offers it.

Interfaces: a SocketCAN interface (can0, vcan0; Linux), a serial-line CAN
adapter as slcan:<device> (slcan:/dev/ttyACM0, slcan:COM3) or
slcan:tcp://host:port, opened at --slcan-bitrate (passive commands open it
listen-only), or "virtual", an in-process bus for tests and simulation.

Results go to stdout as text (default), a table or JSON (--format); progress
and diagnostics go to stderr. Exit codes: docs/exitcodes.md.

Usage:
  canopenctl [command]

Available Commands:
  boot        Boot a network as a CiA 302 manager: identity checks, configuration, start (needs --yes off virtual interfaces)
  cdcf        Concise DCFs (CiA 302-3): convert a DCF, inspect, apply to a node
  cia402      Operate a CiA 402 drive over SDO: state, enable, disable, mode, move, velocity
  completion  Generate a shell completion script
  dcf         Apply a DCF to a node, or export a node's configuration as a DCF
  decode      Decode a trace file as CANopen messages
  eds         Validate, show, convert and generate code from EDS and DCF files (CiA 306)
  emcy        Watch emergency messages
  heartbeat   Watch the heartbeats of the network
  help        Help about any command
  lss         Layer setting services (CiA 305): node-IDs and bit rates of devices, Fastscan
  monitor     Decode live traffic as CANopen messages (passive)
  mpdo        Multiplexed PDOs (CiA 301): configure a node's MPDO and its lists, watch MPDO traffic
  nmt         Send an NMT command to a node or to all nodes (needs --yes off virtual interfaces)
  pdo         Show, map and watch a node's process data objects
  program     Program download (CiA 302-3): replace a node's program, control and read its state
  restore     Make a node restore its default parameters (0x1011, needs --yes off virtual interfaces)
  scan        Find the nodes of a network: device type and identity of each (SDO reads)
  sdo         Read and write a node's object dictionary (SDO client)
  simulate    Run a simulated CANopen node (from an EDS, optionally a CiA 402 drive)
  store       Make a node save its parameters (0x1010, needs --yes off virtual interfaces)
  sync        Produce SYNC messages (needs --yes off virtual interfaces)
  time        Send the TIME stamp (needs --yes off virtual interfaces)
  version     Print the version of canopenctl, go-canopen and go-canbus

Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
  -h, --help                help for canopenctl
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)

Use "canopenctl [command] --help" for more information about a command.
```

## canopenctl monitor

```text
monitor receives frames and decodes them as CANopen messages by the predefined
connection set: NMT, SYNC, TIME, EMCY, PDOs, SDO requests and responses (with
segmented and block transfers followed, so segments name their object),
heartbeat, boot-up, node guarding and LSS. With --fd it also receives CAN FD
frames and decodes the CANopen FD services (CiA 1301): USDO requests and
responses, 20-byte FD emergencies, and FD PDOs, heartbeats and NMT. It never
transmits; slcan adapters are opened listen-only. --node and --service select messages; JSON output is
JSON Lines, one record per message and a summary at the end.

Usage:
  canopenctl monitor [flags]

Examples:
  canopenctl monitor -i can0
  canopenctl monitor -i vcan0 --node 5 --service sdo,emcy --format json

Flags:
  -n, --count int             Stop after this many messages shown (0: no limit)
      --duration duration     Stop after this time (0: until Ctrl-C)
      --fd                    Open the interface for CAN FD and decode CANopen FD (USDO, FD EMCY)
  -h, --help                  help for monitor
  -i, --interface string      CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --node string           Only messages of (or addressed to) these nodes: 5, 1-10,20
      --service stringArray   Only these services (repeatable, comma-separated): emcy, heartbeat, lss, nmt, pdo, rpdo, sdo, sync, time, tpdo, unknown, usdo

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl decode

```text
decode reads a trace (candump log, JSON Lines, pcap, pcapng, ASC, BLF or PEAK
TRC; the format is detected from the content; "-" reads stdin) and decodes its
frames as CANopen messages, as monitor does (CAN FD frames as CANopen FD). Malformed CANopen messages (a
wrong length, say) are shown with the problem and make the exit code 1.

Usage:
  canopenctl decode <trace> [flags]

Examples:
  canopenctl decode trip.log
  canopenctl decode capture.pcapng --node 5 --format json

Flags:
  -h, --help                  help for decode
      --ignore-malformed      Exit 0 even when malformed messages were found
      --node string           Only messages of (or addressed to) these nodes: 5, 1-10,20
      --service stringArray   Only these services (repeatable, comma-separated): emcy, heartbeat, lss, nmt, pdo, rpdo, sdo, sync, time, tpdo, unknown, usdo

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl sdo

```text
The sdo commands are an SDO client (CiA 301): expedited, segmented and block
transfer (--block). With --fd they are a USDO client of CANopen FD (CiA 1301)
instead: expedited and segmented transfers in CAN FD frames, --block selecting
bulk downloads; --client-id is canopenctl's USDO node-ID (default 127). A
device that refuses a transfer answers with an abort code: it is printed with
its meaning and ends the command with exit code 11 (JSON: the error's "abort"
object; a USDO abort is reported with its SDO equivalent and "usdoCode"). Indices are hexadecimal (1018, 0x1018);
sub-indices decimal or 0x hexadecimal.

Usage:
  canopenctl sdo [command]

Available Commands:
  dump        Read every readable object an EDS lists from a node
  read        Read (upload) a sub-object of a node
  write       Write (download) a sub-object of a node (needs --yes off virtual interfaces)

Flags:
  -h, --help   help for sdo

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)

Use "canopenctl sdo [command] --help" for more information about a command.
```

## canopenctl sdo read

```text
read uploads a sub-object and prints its value. The data type comes from
--type, else from --eds, else it is guessed from the length (an unsigned
integer for 1..8 bytes, text for printable data, bytes otherwise; JSON says
"typeSource": "guess"). --output writes the raw data to a file (DOMAIN
objects). read only sends SDO read requests: it needs no --yes.

Usage:
  canopenctl sdo read <node> <index> <sub> [flags]

Examples:
  canopenctl sdo read 5 0x1018 1 -i can0
  canopenctl sdo read 5 1008 0 --type string -i vcan0
  canopenctl sdo read 5 0x2010 0 --block -o firmware.bin -i can0

Flags:
      --block              Use SDO block transfer
      --block-size int     Segments per block the client asks for in block uploads (1..127) (default 127)
      --brs                With --fd: send CAN FD frames with bit rate switching
      --client-id int      With --fd: node-ID of canopenctl's USDO client (its requests go out on 0x600 + it) (default 127)
      --eds string         EDS or DCF of the node: names and data types
      --fd                 CANopen FD (CiA 1301): open the interface for CAN FD and read over USDO instead of SDO
  -h, --help               help for read
  -i, --interface string   CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
  -o, --output string      Also write the raw data to this file
      --retries int        Repeat a transfer that timed out up to this many times (0..10)
      --timeout duration   SDO response timeout (default 1s)
  -t, --type string        Data type: UNSIGNED32, INTEGER16, REAL32, VISIBLE_STRING, OCTET_STRING, DOMAIN... or u32, i16, f32, string, octets

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl sdo write

```text
write downloads a value to a sub-object. The data type comes from --type or
--eds (one is required). Integers are decimal, 0x hexadecimal or octal with a
leading 0 (negative ones with a sign; $NODEID+0x180 is evaluated for the
node); floats decimal; BOOLEAN 0/1/true/false; strings as text; OCTET_STRING
and DOMAIN as hexadecimal digit pairs. --file downloads a file's content
instead of a value (DOMAIN; use --block for large files). --verify reads the
value back and exits 1 when it differs.

Usage:
  canopenctl sdo write <node> <index> <sub> [value] [flags]

Examples:
  canopenctl sdo write 5 0x1017 0 1000 --type u16 -i vcan0
  canopenctl sdo write 5 0x6200 1 0xFF --eds io401.eds -i can0 --yes
  canopenctl sdo write 5 0x1F50 1 --file firmware.bin --block -i can0 --yes

Flags:
      --block              Use SDO block transfer
      --block-size int     Segments per block the client asks for in block uploads (1..127) (default 127)
      --brs                With --fd: send CAN FD frames with bit rate switching
      --client-id int      With --fd: node-ID of canopenctl's USDO client (its requests go out on 0x600 + it) (default 127)
      --eds string         EDS or DCF of the node: data types and names
      --fd                 CANopen FD (CiA 1301): open the interface for CAN FD and write over USDO instead of SDO (--block: bulk transfer)
      --file string        Download this file's content instead of a value
  -h, --help               help for write
  -i, --interface string   CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --retries int        Repeat a transfer that timed out up to this many times (0..10)
      --timeout duration   SDO response timeout (default 1s)
  -t, --type string        Data type: UNSIGNED32, INTEGER16, REAL32, VISIBLE_STRING, OCTET_STRING, DOMAIN... or u32, i16, f32, string, octets
      --verify             Read the value back and compare (exit 1 when it differs)
      --yes                Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl sdo dump

```text
dump reads every readable sub-object the EDS describes (or those of --range)
from the node and prints them with their names and decoded values. Objects
the node refuses are listed with the abort code; any failure makes the exit
code 1. When the node does not answer at all, dump stops with a timeout (exit
5).

Usage:
  canopenctl sdo dump <node> --eds <file> [flags]

Examples:
  canopenctl sdo dump 5 --eds io401.eds -i can0
  canopenctl sdo dump 5 --eds drive.eds --range 6000-6FFF --format table -i vcan0

Flags:
      --block              Use SDO block transfer
      --block-size int     Segments per block the client asks for in block uploads (1..127) (default 127)
      --brs                With --fd: send CAN FD frames with bit rate switching
      --client-id int      With --fd: node-ID of canopenctl's USDO client (its requests go out on 0x600 + it) (default 127)
      --eds string         EDS or DCF of the node (required)
      --fd                 CANopen FD (CiA 1301): open the interface for CAN FD and read over USDO instead of SDO
  -h, --help               help for dump
  -i, --interface string   CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --range string       Only objects in this index range: 6000-6FFF, or one index
      --retries int        Repeat a transfer that timed out up to this many times (0..10)
      --timeout duration   SDO response timeout (default 1s)

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl nmt

```text
nmt sends an NMT module control command (CiA 301): start (operational), stop
(stopped), preop (pre-operational), reset-node (reset application) or
reset-comm (reset communication), to one node (1..127) or to all (all, 0).
--wait waits until the node confirms: its heartbeat reports the new state
(start, stop, preop; the node must produce heartbeats), or its boot-up
message arrives (resets); not with all.

Usage:
  canopenctl nmt <start|stop|preop|reset-node|reset-comm> <node|all> [flags]

Examples:
  canopenctl nmt start all -i vcan0
  canopenctl nmt reset-node 5 --wait -i can0 --yes

Flags:
      --brs                With --fd: send CAN FD frames with bit rate switching
      --fd                 CANopen FD (CiA 1301): open the interface for CAN FD and send the command in a CAN FD frame (--wait: FD heartbeats)
  -h, --help               help for nmt
  -i, --interface string   CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --timeout duration   How long --wait waits (default 5s)
      --wait               Wait for the node's heartbeat (or boot-up) to confirm
      --yes                Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl heartbeat

```text
Watch the heartbeats of the network

Usage:
  canopenctl heartbeat [command]

Available Commands:
  watch       Report boot-ups, NMT state changes and heartbeat timeouts (passive)

Flags:
  -h, --help   help for heartbeat

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)

Use "canopenctl heartbeat [command] --help" for more information about a command.
```

## canopenctl heartbeat watch

```text
watch receives the NMT error control messages (heartbeat, boot-up, node
guarding responses) of the network and reports each node's boot-ups and state
changes. With --supervise every node (or those of --node) is supervised from
its first heartbeat: a node silent for longer is reported lost once, and
again when it resumes. It never transmits. At the end a summary lists the
nodes; a heartbeat timeout during the watch makes the exit code 1.

Usage:
  canopenctl heartbeat watch [flags]

Examples:
  canopenctl heartbeat watch -i can0 --supervise 1500ms --duration 1m
  canopenctl heartbeat watch -i vcan0 --node 5,6 --format json

Flags:
      --brs                  With --fd: send CAN FD frames with bit rate switching
  -n, --count int            Stop after this many events (0: no limit)
      --duration duration    Stop after this time (0: until Ctrl-C)
      --fd                   CANopen FD (CiA 1301): open the interface for CAN FD and accept CAN FD heartbeats
  -h, --help                 help for watch
  -i, --interface string     CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --node string          Only these nodes: 5, 1-10,20
      --supervise duration   Consumer heartbeat time: report a node lost after this silence (0: no supervision)

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl scan

```text
scan reads the device type (0x1000) and the identity (0x1018: vendor-ID,
product code, revision, serial number) of every node-ID 1..127 (or --nodes)
by SDO, --parallel at a time with a short --timeout, and lists the nodes that
answered. It only sends SDO read requests (one per node-ID and object): it
needs no --yes. A node that answers 0x1000 but fails on 0x1018 is listed under
errors and makes the exit code 1.

Usage:
  canopenctl scan [flags]

Examples:
  canopenctl scan -i can0
  canopenctl scan -i vcan0 --nodes 1-16 --format table

Flags:
      --brs                With --fd: send CAN FD frames with bit rate switching
      --fd                 CANopen FD (CiA 1301): open the interface for CAN FD and read over USDO instead of SDO
  -h, --help               help for scan
  -i, --interface string   CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --nodes string       Node-IDs to scan: 1-127, 1-10,20 (default "1-127")
      --parallel int       Nodes queried at once (default 16)
      --timeout duration   SDO response timeout per node (default 100ms)

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl emcy

```text
Watch emergency messages

Usage:
  canopenctl emcy [command]

Available Commands:
  watch       Print the emergency messages of the network, decoded (passive)

Flags:
  -h, --help   help for emcy

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)

Use "canopenctl emcy [command] --help" for more information about a command.
```

## canopenctl emcy watch

```text
watch receives the emergency messages (EMCY, 0x081..0x0FF) of every node, or
of --node, and prints each with the meaning of its error code (CiA 301 table
21), the error register bits and the manufacturer-specific field. An error
code 0x0000 is an error reset. It never transmits.

Usage:
  canopenctl emcy watch [flags]

Examples:
  canopenctl emcy watch -i can0
  canopenctl emcy watch -i vcan0 --node 5 --count 1 --format json

Flags:
      --brs                 With --fd: send CAN FD frames with bit rate switching
  -n, --count int           Stop after this many messages (0: no limit)
      --duration duration   Stop after this time (0: until Ctrl-C)
      --fd                  CANopen FD (CiA 1301): open the interface for CAN FD and decode 20-byte CANopen FD emergencies
  -h, --help                help for watch
  -i, --interface string    CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --node string         Only these nodes: 5, 1-10,20

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl sync

```text
sync sends SYNC messages (CiA 301) every --period, --count times (default 10)
or until --duration. --counter adds the SYNC counter byte running from 1 to
the overflow value (2..240). Synchronous PDOs of the nodes are triggered by
it: production is a state change of the network and needs --yes on a real
interface. Timing is that of a Go ticker (jitter of tens to hundreds of
microseconds): use it for tests and commissioning, not as a precision clock.

Usage:
  canopenctl sync [flags]

Examples:
  canopenctl sync -i vcan0 --period 10ms --count 100
  canopenctl sync -i can0 --period 1ms --duration 10s --counter 16 --yes

Flags:
      --brs                 With --fd: send CAN FD frames with bit rate switching
      --cob-id string       COB-ID of the SYNC message (hexadecimal) (default "80")
  -n, --count int           Send this many messages (0: until --duration) (default 10)
      --counter int         SYNC counter overflow value: 0 (no counter) or 2..240
      --duration duration   Stop after this time (0: no limit; needed with --count 0)
      --fd                  CANopen FD (CiA 1301): open the interface for CAN FD and send the SYNC in CAN FD frames
  -h, --help                help for sync
  -i, --interface string    CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --period duration     Time between two messages (at least 1ms) (default 100ms)
      --yes                 Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl time

```text
time sends a TIME message (CiA 301 TIME_OF_DAY: milliseconds after midnight
and days since 1984-01-01, UTC) with the current time, or --at. With --count
it repeats every --period (the current time each time unless --at is given).

Usage:
  canopenctl time [flags]

Examples:
  canopenctl time -i vcan0
  canopenctl time -i can0 --at 2026-10-29T12:00:00Z --yes

Flags:
      --at string           Send this time (RFC 3339) instead of the current time
      --cob-id string       COB-ID of the TIME message (hexadecimal) (default "100")
  -n, --count int           Send this many messages (0: until --duration) (default 1)
      --duration duration   Stop after this time (0: no limit; needed with --count 0)
  -h, --help                help for time
  -i, --interface string    CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --period duration     Time between two messages (at least 1ms) (default 1s)
      --yes                 Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl pdo

```text
Show, map and watch a node's process data objects

Usage:
  canopenctl pdo [command]

Available Commands:
  map         Configure a PDO's mapping and communication parameters (needs --yes off virtual interfaces)
  show        Read a node's PDO communication and mapping parameters (SDO reads)
  watch       Print a node's TPDO with its decoded values

Flags:
  -h, --help   help for pdo

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)

Use "canopenctl pdo [command] --help" for more information about a command.
```

## canopenctl pdo show

```text
show reads the communication (0x1400.., 0x1800..) and mapping (0x1600..,
0x1A00..) parameters of the node's PDOs by SDO: TPDO 1..4 and RPDO 1..4, or
the one of --tpdo or --rpdo. PDOs the node does not have are left out (with
--tpdo/--rpdo they are an error). --eds names the mapped objects. It only
sends SDO read requests.

Usage:
  canopenctl pdo show <node> [flags]

Examples:
  canopenctl pdo show 5 -i can0
  canopenctl pdo show 5 --tpdo 1 --eds io401.eds -i vcan0

Flags:
      --brs                With --fd: send CAN FD frames with bit rate switching
      --client-id int      With --fd: node-ID of canopenctl's USDO client (its requests go out on 0x600 + it) (default 127)
      --eds string         EDS or DCF of the node: names of the mapped objects
      --fd                 CANopen FD (CiA 1301): open the interface for CAN FD and read over USDO instead of SDO
  -h, --help               help for show
  -i, --interface string   CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --retries int        Repeat a transfer that timed out up to this many times (0..10)
      --rpdo int           Only this RPDO (1..512)
      --timeout duration   SDO response timeout (default 1s)
      --tpdo int           Only this TPDO (1..512)

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl pdo map

```text
map writes a PDO's configuration by SDO in the order CiA 301 requires:
invalidate the COB-ID, clear the mapping, write the entries, enable the
mapping, the transmission type, inhibit time, event timer and SYNC start
value, and make the COB-ID valid. Parameters not given keep the node's
values. Entries are IIII:SS:BITS (index and
sub-index hexadecimal, length in bits), comma-separated; dummy entries
(0002..0007:00) skip bits of an RPDO. The COB-ID is kept from the node unless
--cob-id is given; --disable leaves the PDO invalid. --eds checks the mapping
against the node's description before anything is sent. Most devices accept
mapping changes only in pre-operational.

Usage:
  canopenctl pdo map <node> (--tpdo n | --rpdo n) --map IIII:SS:BITS,... [flags]

Examples:
  canopenctl pdo map 5 --tpdo 1 --map 0x6041:00:16,0x6064:00:32 --type 255 --event-timer 100 -i vcan0
  canopenctl pdo map 5 --rpdo 1 --map 6200:01:8,6200:02:8 --type 1 -i can0 --yes
  canopenctl pdo map 5 --tpdo 2 --disable -i can0 --yes

Flags:
      --brs                With --fd: send CAN FD frames with bit rate switching
      --client-id int      With --fd: node-ID of canopenctl's USDO client (its requests go out on 0x600 + it) (default 127)
      --cob-id string      CAN identifier of the PDO (hexadecimal; default: keep the node's)
      --disable            Leave the PDO invalid (COB-ID bit 31)
      --eds string         EDS or DCF of the node: check the mapping before sending
      --event-timer int    Event timer (TPDO) or deadline (RPDO) in ms (default: keep the node's)
      --fd                 CANopen FD (CiA 1301): open the interface for CAN FD and write over USDO instead of SDO
  -h, --help               help for map
      --inhibit int        Inhibit time of a TPDO in 100 µs (default: keep the node's)
  -i, --interface string   CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --map string         Mapping entries IIII:SS:BITS, comma-separated
      --no-rtr             Set COB-ID bit 30: no remote transmission requests
      --retries int        Repeat a transfer that timed out up to this many times (0..10)
      --rpdo int           Configure this RPDO (1..512)
      --sync-start int     SYNC start value of a TPDO, 0..240 (default: keep the node's)
      --timeout duration   SDO response timeout (default 1s)
      --tpdo int           Configure this TPDO (1..512)
      --type int           Transmission type: 0 acyclic sync, 1..240 every nth SYNC, 252/253 RTR (TPDO), 254/255 event-driven (default: keep the node's) (default 255)
      --yes                Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl pdo watch

```text
watch reads the TPDO's COB-ID and mapping from the node by SDO (or takes them
from --cob-id and --map, sending nothing), then receives the PDO and prints
each one with its values. --eds gives the names and data types of the mapped
objects; without it every value is an unsigned integer of its length. The
node must be operational to transmit.

Usage:
  canopenctl pdo watch <node> --tpdo n [flags]

Examples:
  canopenctl pdo watch 5 --tpdo 1 --eds io401.eds -i can0
  canopenctl pdo watch 5 --tpdo 1 --cob-id 185 --map 6000:01:8,6000:02:8 -i can0 --format json

Flags:
      --cob-id string       COB-ID of the PDO (hexadecimal; with --map: read nothing from the node)
  -n, --count int           Stop after this many PDOs (0: no limit)
      --duration duration   Stop after this time (0: until Ctrl-C)
      --eds string          EDS or DCF of the node: names and data types of the mapped objects
  -h, --help                help for watch
  -i, --interface string    CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --map string          Mapping IIII:SS:BITS,... (with --cob-id)
      --retries int         Repeat a transfer that timed out up to this many times (0..10)
      --timeout duration    SDO response timeout (default 1s)
      --tpdo int            TPDO number (1..512, required)

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl eds

```text
Validate, show, convert and generate code from EDS and DCF files (CiA 306)

Usage:
  canopenctl eds [command]

Available Commands:
  convert     Convert an EDS to a node's DCF, or a DCF back to an EDS
  generate    Generate typed Go code (constants, dictionary, accessors) from an EDS
  show        List the device information and the objects of an EDS or DCF
  validate    Check an EDS or DCF against CiA 306 and CiA 301

Flags:
  -h, --help   help for eds

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)

Use "canopenctl eds [command] --help" for more information about a command.
```

## canopenctl eds validate

```text
validate checks an EDS or DCF: mandatory objects, object lists, object, data
and access types, sub-object counts, default, parameter and limit values
against their data types, PDO mapping flags, PDO counts, compact storage and
DCF commissioning values. Errors (or warnings with --strict) make the exit
code 1; a file that cannot be read at all exits 3.

Usage:
  canopenctl eds validate <file> [flags]

Examples:
  canopenctl eds validate device.eds
  canopenctl eds validate node5.dcf --strict --format json

Flags:
  -h, --help     help for validate
      --strict   Exit 1 on warnings too

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl eds show

```text
show prints the device information and the object table of an EDS or DCF:
every sub-object with its name, data type, access type, PDO mappability,
value (the DCF's ParameterValue, else the default; $NODEID values evaluated
for --node-id when given) and limits. Objects a dictionary cannot hold are
left out (eds validate reports them).

Usage:
  canopenctl eds show <file> [flags]

Examples:
  canopenctl eds show device.eds
  canopenctl eds show device.eds --range 6000-6FFF --format json

Flags:
  -h, --help           help for show
      --node-id int    Evaluate $NODEID values for this node-ID (0: show them as $NODEID+...)
      --range string   Only objects in this index range: 6000-6FFF, or one index

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl eds convert

```text
convert writes a DCF for a node from an EDS (or re-targets a DCF to another
node): a [DeviceComissioning] section with --node-id (and --node-name,
--baud), and every sub-object's value as ParameterValue, with $NODEID
evaluated for the node. --to eds removes the commissioning section and the
parameter values. The target format is --to, else the extension of --output
(.dcf, .eds). The output is CiA 306 in a canonical layout (comments are not
kept); without --output it goes to stdout.

Usage:
  canopenctl eds convert <file> [flags]

Examples:
  canopenctl eds convert io401.eds --node-id 5 -o node5.dcf
  canopenctl eds convert node5.dcf --to eds -o io401.eds

Flags:
      --baud int           Baudrate (kbit/s) of the DCF's commissioning section
  -h, --help               help for convert
      --node-id int        Node-ID of the DCF (default: the input DCF's)
      --node-name string   NodeName of the DCF's commissioning section
  -o, --output string      Output file (default: stdout)
      --to string          Target: dcf or eds (default: from the --output extension)

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl eds generate

```text
generate writes a Go source file for an EDS or DCF: index and sub-index
constants, NewDictionary (the object dictionary without parsing at run time),
a typed local accessor and a typed remote accessor over an SDO client. The
output is deterministic and gofmt'd and names the source file and its
SHA-256. The file must validate without errors.

Usage:
  canopenctl eds generate <file> --package <name> [flags]

Examples:
  canopenctl eds generate io401.eds --package io401 -o io401/io401.go

Flags:
  -h, --help             help for generate
  -o, --output string    Output file (default: stdout)
      --package string   Go package name (required)

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl dcf

```text
Apply a DCF to a node, or export a node's configuration as a DCF

Usage:
  canopenctl dcf [command]

Available Commands:
  apply       Write a DCF's configuration to a node (needs --yes off virtual interfaces)
  export      Read a node's current values into a DCF (SDO reads)

Flags:
  -h, --help   help for dcf

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)

Use "canopenctl dcf [command] --help" for more information about a command.
```

## canopenctl dcf apply

```text
apply writes every writable sub-object a DCF configures (ParameterValue,
without the "refuse write on download" flag) to the node by SDO, with
$NODEID values evaluated for the node. PDO parameters are applied per PDO in
the order CiA 301 requires (invalidate, clear the mapping, map, enable, make
valid), however the DCF orders them. The first refused write stops it (exit
11, the abort code and object in the error). --dry-run lists the entries
without sending anything. <node> 0 takes the DCF's node-ID.

Usage:
  canopenctl dcf apply <node> <file> [flags]

Examples:
  canopenctl dcf apply 5 node5.dcf -i vcan0
  canopenctl dcf apply 5 node5.dcf --dry-run
  canopenctl dcf apply 0 node5.dcf -i can0 --yes

Flags:
      --brs                With --fd: send CAN FD frames with bit rate switching
      --client-id int      With --fd: node-ID of canopenctl's USDO client (its requests go out on 0x600 + it) (default 127)
      --dry-run            List the entries without sending anything
      --fd                 CANopen FD (CiA 1301): open the interface for CAN FD and write over USDO instead of SDO
  -h, --help               help for apply
  -i, --interface string   CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --retries int        Repeat a transfer that timed out up to this many times (0..10)
      --timeout duration   SDO response timeout (default 1s)
      --yes                Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl dcf export

```text
export reads every readable sub-object the EDS describes from the node and
writes a DCF holding the values read as ParameterValue (objects the node
refuses keep the EDS default and are reported on stderr; the pre-defined
error field 0x1003, diagnostics rather than configuration, keeps it too, so
that dcf apply can write the DCF back). The DCF's
commissioning section gets the node-ID. It only sends SDO read requests.
Without --output the DCF goes to stdout.

Usage:
  canopenctl dcf export <node> --eds <file> [flags]

Examples:
  canopenctl dcf export 5 --eds io401.eds -o node5.dcf -i can0

Flags:
      --block              Use SDO block transfer
      --block-size int     Segments per block the client asks for in block uploads (1..127) (default 127)
      --eds string         EDS of the node (required)
  -h, --help               help for export
  -i, --interface string   CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --node-name string   NodeName of the DCF's commissioning section
  -o, --output string      Output file (default: stdout)
      --retries int        Repeat a transfer that timed out up to this many times (0..10)
      --timeout duration   SDO response timeout (default 1s)

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl cia402

```text
The cia402 commands operate a CiA 402 drive (IEC 61800-7-201) through its SDO
server: the controlword (0x6040) is written and the statusword (0x6041) read
back to verify every step of the state machine. Everything but state moves or
powers the drive: those commands need --yes on a real interface. A drive in
Fault, or one that does not reach a state within --step-timeout, ends the
command with exit code 12 (or 5) and the drive's state in the error.

Usage:
  canopenctl cia402 [command]

Available Commands:
  disable     Bring the drive to Switch on disabled (power stage off) (needs --yes off virtual interfaces)
  enable      Bring the drive to Operation enabled (needs --yes off virtual interfaces)
  fault-reset Reset a fault (rising edge of controlword bit 7) (needs --yes off virtual interfaces)
  mode        Set the mode of operation (pp, pv, pt, hm, csp, csv, cst...) (needs --yes off virtual interfaces)
  move        Profile position move to a target (increments) (needs --yes off virtual interfaces)
  quick-stop  Send the quick stop command (needs --yes off virtual interfaces)
  state       Read the drive's state, mode, position, velocity and fault code
  velocity    Profile velocity: set the target velocity (increments/s) (needs --yes off virtual interfaces)

Flags:
  -h, --help   help for cia402

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)

Use "canopenctl cia402 [command] --help" for more information about a command.
```

## canopenctl cia402 state

```text
state reads the statusword and decodes the state, with the mode of operation, the actual position and velocity and, in Fault, the error code (0x603F). It only sends SDO read requests.

Usage:
  canopenctl cia402 state <node> [flags]

Flags:
  -h, --help                    help for state
  -i, --interface string        CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --poll duration           Pause between two statusword reads (default 10ms)
      --retries int             Repeat a transfer that timed out up to this many times (0..10)
      --step-timeout duration   Bound of every step: a state transition, a mode change, a set-point handshake, a move (default 2s)
      --timeout duration        SDO response timeout (default 1s)

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl cia402 enable

```text
enable walks the state machine to Operation enabled, verifying each state. A drive in Fault fails (exit 12) unless --reset-fault.

Usage:
  canopenctl cia402 enable <node> [flags]

Flags:
  -h, --help                    help for enable
  -i, --interface string        CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --poll duration           Pause between two statusword reads (default 10ms)
      --reset-fault             Reset a fault on the way
      --retries int             Repeat a transfer that timed out up to this many times (0..10)
      --step-timeout duration   Bound of every step: a state transition, a mode change, a set-point handshake, a move (default 2s)
      --timeout duration        SDO response timeout (default 1s)
      --yes                     Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl cia402 disable

```text
Bring the drive to Switch on disabled (power stage off) (needs --yes off virtual interfaces)

Usage:
  canopenctl cia402 disable <node> [flags]

Flags:
  -h, --help                    help for disable
  -i, --interface string        CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --poll duration           Pause between two statusword reads (default 10ms)
      --retries int             Repeat a transfer that timed out up to this many times (0..10)
      --step-timeout duration   Bound of every step: a state transition, a mode change, a set-point handshake, a move (default 2s)
      --timeout duration        SDO response timeout (default 1s)
      --yes                     Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl cia402 fault-reset

```text
Reset a fault (rising edge of controlword bit 7) (needs --yes off virtual interfaces)

Usage:
  canopenctl cia402 fault-reset <node> [flags]

Flags:
  -h, --help                    help for fault-reset
  -i, --interface string        CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --poll duration           Pause between two statusword reads (default 10ms)
      --retries int             Repeat a transfer that timed out up to this many times (0..10)
      --step-timeout duration   Bound of every step: a state transition, a mode change, a set-point handshake, a move (default 2s)
      --timeout duration        SDO response timeout (default 1s)
      --yes                     Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl cia402 quick-stop

```text
Send the quick stop command (needs --yes off virtual interfaces)

Usage:
  canopenctl cia402 quick-stop <node> [flags]

Flags:
  -h, --help                    help for quick-stop
  -i, --interface string        CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --poll duration           Pause between two statusword reads (default 10ms)
      --retries int             Repeat a transfer that timed out up to this many times (0..10)
      --step-timeout duration   Bound of every step: a state transition, a mode change, a set-point handshake, a move (default 2s)
      --timeout duration        SDO response timeout (default 1s)
      --yes                     Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl cia402 mode

```text
Set the mode of operation (pp, pv, pt, hm, csp, csv, cst...) (needs --yes off virtual interfaces)

Usage:
  canopenctl cia402 mode <node> <mode> [flags]

Flags:
  -h, --help                    help for mode
  -i, --interface string        CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --poll duration           Pause between two statusword reads (default 10ms)
      --retries int             Repeat a transfer that timed out up to this many times (0..10)
      --step-timeout duration   Bound of every step: a state transition, a mode change, a set-point handshake, a move (default 2s)
      --timeout duration        SDO response timeout (default 1s)
      --yes                     Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl cia402 move

```text
move switches to profile position mode when needed and moves to the target position with the set-point handshake; --wait waits for target reached. The drive must be in Operation enabled. Put -- before a negative target: cia402 move -i can0 --yes -- 3 -500.

Usage:
  canopenctl cia402 move <node> <target> [flags]

Flags:
      --accel uint32            Profile acceleration 0x6083 (0: keep)
      --decel uint32            Profile deceleration 0x6084 (0: keep)
  -h, --help                    help for move
      --immediate               Replace an active set-point (change set immediately)
  -i, --interface string        CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --move-timeout duration   Bound of the wait for target reached (0: --step-timeout)
      --poll duration           Pause between two statusword reads (default 10ms)
      --relative                Target relative to the preceding target
      --retries int             Repeat a transfer that timed out up to this many times (0..10)
      --step-timeout duration   Bound of every step: a state transition, a mode change, a set-point handshake, a move (default 2s)
      --timeout duration        SDO response timeout (default 1s)
      --velocity uint32         Profile velocity 0x6081 (0: keep)
      --wait                    Wait for target reached
      --yes                     Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl cia402 velocity

```text
velocity switches to profile velocity mode when needed and writes the target velocity (0x60FF); it does not wait for the drive to reach it. The drive must be in Operation enabled. Put -- before a negative velocity.

Usage:
  canopenctl cia402 velocity <node> <velocity> [flags]

Flags:
  -h, --help                    help for velocity
  -i, --interface string        CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --poll duration           Pause between two statusword reads (default 10ms)
      --retries int             Repeat a transfer that timed out up to this many times (0..10)
      --step-timeout duration   Bound of every step: a state transition, a mode change, a set-point handshake, a move (default 2s)
      --timeout duration        SDO response timeout (default 1s)
      --yes                     Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl lss

```text
The lss commands are an LSS master (CiA 305, on 0x7E5/0x7E4). Devices are
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
refusal by the device exit 12 (lss_refused).

Usage:
  canopenctl lss [command]

Available Commands:
  activate    Make the slaves in the configuration state switch to their configured bit rate
  assign-all  Give every unconfigured device a node-ID (Fastscan, configure node-ID)
  configure   Configure the node-ID and bit rate of the slave in the configuration state, and store them
  fastscan    List the LSS addresses of the unconfigured devices (Fastscan)
  identify    Ask whether a device without node-ID (or with an LSS address) is on the network
  inquire     Read the LSS address and node-ID of the slave in the configuration state
  select      Switch the slave with an LSS address to the configuration state (switch state selective)
  switch      Switch every LSS slave to the waiting or the configuration state (switch state global)

Flags:
  -h, --help   help for lss

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)

Use "canopenctl lss [command] --help" for more information about a command.
```

## canopenctl lss switch

```text
switch sends "switch state global" (unconfirmed): every LSS slave enters the
waiting or the configuration state. A device whose node-ID was configured
while it had none applies it when it is switched to waiting.

Usage:
  canopenctl lss switch <waiting|configuration> [flags]

Examples:
  canopenctl lss switch configuration -i vcan0
  canopenctl lss switch waiting -i can0 --yes

Flags:
  -h, --help               help for switch
  -i, --interface string   CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --retries int        Re-sends of a request without response (0..10) (default 2)
      --timeout duration   Response timeout of a confirmed LSS service (default 1s)
      --window duration    Listening window of each Fastscan query (default 25ms)
      --yes                Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl lss select

```text
select sends "switch state selective" with an LSS address: the slave that has
it enters the configuration state and confirms. No answer is exit 5. Slaves
already in the configuration state stay there; "lss switch waiting" ends
the configuration.

Usage:
  canopenctl lss select <address> [flags]

Examples:
  canopenctl lss select 00000F7A:0000C0C7:00010000:00000005 -i vcan0

Flags:
  -h, --help               help for select
  -i, --interface string   CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --retries int        Re-sends of a request without response (0..10) (default 2)
      --timeout duration   Response timeout of a confirmed LSS service (default 1s)
      --window duration    Listening window of each Fastscan query (default 25ms)
      --yes                Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl lss inquire

```text
inquire reads the four parts of the LSS address (inquire identity) and the
active node-ID (inquire node-ID; 255 when the device has none) of the slave
in the configuration state. Exactly one slave should be in it: select it
first, or give --address. No answer is exit 5.

Usage:
  canopenctl lss inquire [flags]

Examples:
  canopenctl lss select 00000F7A:0000C0C7:00010000:00000005 -i vcan0 && canopenctl lss inquire -i vcan0
  canopenctl lss inquire --address 00000F7A:0000C0C7:00010000:00000005 -i vcan0

Flags:
      --address string     Select the slave with this LSS address first (vendor:product:revision:serial)
  -h, --help               help for inquire
  -i, --interface string   CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --retries int        Re-sends of a request without response (0..10) (default 2)
      --timeout duration   Response timeout of a confirmed LSS service (default 1s)
      --window duration    Listening window of each Fastscan query (default 25ms)
      --yes                Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl lss configure

```text
configure sends, to the slave in the configuration state (or, with --address,
to that slave, selected for it), in this order: configure node-ID
(--node-id 1..127, or "none" to remove it), configure bit timing (--bit-rate
in kbit/s from the CiA 301 table, or auto; it takes effect with "lss
activate") and store configuration (--store: the device keeps both across
power cycles). At least one is needed. A refusal is exit 12 with the
device's error code.

The new node-ID is pending: a device that had none applies it when it
leaves the configuration state ("lss switch waiting", or at once with
--address), a configured device at its next NMT reset.

Usage:
  canopenctl lss configure [flags]

Examples:
  canopenctl lss configure --address 00000F7A:0000C0C7:00010000:00001001 --node-id 5 --store -i vcan0
  canopenctl lss configure --bit-rate 250 -i can0 --yes

Flags:
      --address string     Select the slave with this LSS address for the configuration (vendor:product:revision:serial)
      --bit-rate string    Bit rate to configure in kbit/s (1000, 800, 500, 250, 125, 50, 20, 10) or auto
  -h, --help               help for configure
  -i, --interface string   CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --node-id string     Node-ID to configure: 1..127, or none
      --retries int        Re-sends of a request without response (0..10) (default 2)
      --store              Make the slave store its node-ID and bit rate
      --timeout duration   Response timeout of a confirmed LSS service (default 1s)
      --window duration    Listening window of each Fastscan query (default 25ms)
      --yes                Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl lss activate

```text
activate sends "activate bit timing parameters" (unconfirmed): every slave in
the configuration state switches to the bit rate configured with "lss
configure --bit-rate" after --delay, and stays silent for another --delay.
canopenctl's own interface keeps its bit rate: change it yourself (ip link,
--slcan-bitrate) within the delay.

Usage:
  canopenctl lss activate [flags]

Examples:
  canopenctl lss activate --delay 500ms -i can0 --yes

Flags:
      --delay duration     Switch delay (0..65535ms) (default 100ms)
  -h, --help               help for activate
  -i, --interface string   CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --retries int        Re-sends of a request without response (0..10) (default 2)
      --timeout duration   Response timeout of a confirmed LSS service (default 1s)
      --window duration    Listening window of each Fastscan query (default 25ms)
      --yes                Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl lss identify

```text
identify sends "identify non-configured remote slave": devices with neither an
active nor a pending node-ID answer. With --address it sends "identify remote
slave" instead: devices whose LSS address is --address (or in the range
--address..--to: vendor-ID and product code equal, revision and serial
number inclusive ranges) answer. The result says whether any device
answered; silence is not an error.

Usage:
  canopenctl lss identify [flags]

Examples:
  canopenctl lss identify -i vcan0
  canopenctl lss identify --address 00000F7A:0000C0C7:00010000:00000000 --to 00000F7A:0000C0C7:00010000:FFFFFFFF -i vcan0

Flags:
      --address string     Identify the devices with this LSS address (the low end with --to)
  -h, --help               help for identify
  -i, --interface string   CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --retries int        Re-sends of a request without response (0..10) (default 2)
      --timeout duration   Response timeout of a confirmed LSS service (default 1s)
      --to string          High end of the LSS address range of --address
      --window duration    Listening window of each Fastscan query (default 25ms)
      --yes                Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl lss fastscan

```text
fastscan finds the LSS addresses of the devices without node-ID with LSS
Fastscan, bit by bit, without selecting any. Every query costs a listening
window (--window): a full scan of one device takes about 130 windows, fewer
with known parts (--vendor, --product, --revision, which are then verified
instead of scanned). Configured devices do not take part: "lss inquire" or
"scan" find those.

Usage:
  canopenctl lss fastscan [flags]

Examples:
  canopenctl lss fastscan -i vcan0
  canopenctl lss fastscan --vendor 0xF7A --product 0xC0C7 --window 10ms -i vcan0 --format table

Flags:
  -h, --help               help for fastscan
  -i, --interface string   CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --max int            Stop after this many devices (0: all)
      --product string     Only devices with this product code
      --retries int        Re-sends of a request without response (0..10) (default 2)
      --revision string    Only devices with this revision number
      --timeout duration   Response timeout of a confirmed LSS service (default 1s)
      --vendor string      Only devices with this vendor-ID (verified instead of scanned: faster)
      --window duration    Listening window of each Fastscan query (default 25ms)
      --yes                Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl lss assign-all

```text
assign-all gives the devices without node-ID node-IDs from --first upwards,
skipping --avoid: one by one, it selects a device with Fastscan, checks that
it alone is in the configuration state, configures its node-ID (and makes it
store it with --store) and switches it back to waiting, upon which the
device applies the node-ID (reset communication) and boots. It ends when no
unconfigured device answers. A node-ID once requested is never offered
again, even when the device did not confirm.

Node-IDs already used by configured devices are not detected: give them
with --avoid (scan finds them). When a step fails after some devices were
configured, the result lists them with the error and the exit code is 1.

Usage:
  canopenctl lss assign-all [flags]

Examples:
  canopenctl lss assign-all --first 10 -i vcan0
  canopenctl lss assign-all --first 2 --avoid 1-9 --store --vendor 0xF7A -i can0 --yes

Flags:
      --avoid string       Node-IDs not to give (in use): 1-9,20
      --first int          First node-ID to give (required)
  -h, --help               help for assign-all
  -i, --interface string   CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --max int            Stop after this many devices (0: all)
      --product string     Only devices with this product code
      --retries int        Re-sends of a request without response (0..10) (default 2)
      --revision string    Only devices with this revision number
      --store              Make every device store its node-ID
      --timeout duration   Response timeout of a confirmed LSS service (default 1s)
      --vendor string      Only devices with this vendor-ID (verified instead of scanned: faster)
      --window duration    Listening window of each Fastscan query (default 25ms)
      --yes                Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl mpdo

```text
An MPDO carries one object per message: in destination address mode (DAM) the
producer writes an object of a consumer (or of all), in source address mode
(SAM) it publishes one of its own objects, listed in its object scanner list
(0x1FA0..), which consumers map to local objects with their object
dispatcher list (0x1FD0..). The mpdo commands configure this over SDO (or
USDO with --fd) and watch the traffic.

Usage:
  canopenctl mpdo [command]

Available Commands:
  configure   Make a TPDO or RPDO of a node a multiplexed PDO (needs --yes off virtual interfaces)
  dispatcher  Write the object dispatcher list of a node (needs --yes off virtual interfaces)
  scanner     Write the object scanner list of a node (needs --yes off virtual interfaces)
  watch       Decode the MPDOs on COB-IDs (passive)

Flags:
  -h, --help   help for mpdo

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)

Use "canopenctl mpdo [command] --help" for more information about a command.
```

## canopenctl mpdo configure

```text
configure makes TPDO or RPDO n of a node an MPDO in destination (--mode dam) or
source address mode (--mode sam), on --cob-id, event-driven (transmission
type 254), with the CiA 301 configuration sequence (invalidate, mapping
sub-index 0 = FEh/FFh, valid). A SAM producer also needs an object scanner
list and its consumers an object dispatcher list ("mpdo scanner", "mpdo
dispatcher"). The device refuses with an abort (exit 11) when it has no MPDO
support.

Usage:
  canopenctl mpdo configure <node> [flags]

Examples:
  canopenctl mpdo configure 5 --tpdo 1 --mode dam --cob-id 0x185 -i vcan0
  canopenctl mpdo configure 6 --rpdo 2 --mode sam --cob-id 0x285 -i can0 --yes

Flags:
      --brs                With --fd: send CAN FD frames with bit rate switching
      --client-id int      With --fd: node-ID of canopenctl's USDO client (its requests go out on 0x600 + it) (default 127)
      --cob-id string      COB-ID of the MPDO (hexadecimal, required)
      --fd                 CANopen FD (CiA 1301): open the interface for CAN FD and write over USDO instead of SDO
  -h, --help               help for configure
      --inhibit int        Inhibit time of a TPDO in 100 µs
  -i, --interface string   CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --mode string        Address mode: dam (destination) or sam (source) (required)
      --retries int        Repeat a transfer that timed out up to this many times (0..10)
      --rpdo int           Make this RPDO an MPDO consumer (1..512)
      --timeout duration   SDO response timeout (default 1s)
      --tpdo int           Make this TPDO an MPDO producer (1..512)
      --yes                Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl mpdo scanner

```text
scanner writes object scanner list --list (0x1FA0 + list - 1) of a node, a
producer of SAM-MPDOs: each --entry IIII:SS[+n] allows the producer to send
object IIII:SS (and n-1 more sub-indices) in SAM-MPDOs. The entries go to
sub-indices 1.., then their number to sub-index 0 (skipped when the list has
a fixed size).

Usage:
  canopenctl mpdo scanner <node> [flags]

Examples:
  canopenctl mpdo scanner 5 --list 1 --entry 2000:00 --entry 2001:01+4 -i vcan0

Flags:
      --brs                 With --fd: send CAN FD frames with bit rate switching
      --client-id int       With --fd: node-ID of canopenctl's USDO client (its requests go out on 0x600 + it) (default 127)
      --entry stringArray   An entry IIII:SS[+n] (repeatable, comma-separated; none clears the list)
      --fd                  CANopen FD (CiA 1301): open the interface for CAN FD and write over USDO instead of SDO
  -h, --help                help for scanner
  -i, --interface string    CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --list int            List number 1..48 (default 1)
      --retries int         Repeat a transfer that timed out up to this many times (0..10)
      --timeout duration    SDO response timeout (default 1s)
      --yes                 Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl mpdo dispatcher

```text
dispatcher writes object dispatcher list --list (0x1FD0 + list - 1) of a node,
a consumer of SAM-MPDOs: each --entry NODE:IIII:SS[+n]=IIII:SS maps object
IIII:SS (and n-1 more sub-indices) of producer NODE to a local object. The
entries go to sub-indices 1.., then their number to sub-index 0 (skipped
when the list has a fixed size).

Usage:
  canopenctl mpdo dispatcher <node> [flags]

Examples:
  canopenctl mpdo dispatcher 6 --list 1 --entry 5:2000:00=2100:00 --entry 5:2001:01+4=2101:01 -i vcan0

Flags:
      --brs                 With --fd: send CAN FD frames with bit rate switching
      --client-id int       With --fd: node-ID of canopenctl's USDO client (its requests go out on 0x600 + it) (default 127)
      --entry stringArray   An entry NODE:IIII:SS[+n]=IIII:SS (repeatable, comma-separated; none clears the list)
      --fd                  CANopen FD (CiA 1301): open the interface for CAN FD and write over USDO instead of SDO
  -h, --help                help for dispatcher
  -i, --interface string    CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --list int            List number 1..48 (default 1)
      --retries int         Repeat a transfer that timed out up to this many times (0..10)
      --timeout duration    SDO response timeout (default 1s)
      --yes                 Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl mpdo watch

```text
watch receives the frames of --cob-id (repeatable) and decodes them as MPDOs:
the address mode, the node (DAM: the consumer, 0 for all; SAM: the
producer), the object and its 4 data bytes. --eds names the objects. It
never transmits. A frame shorter than 8 bytes is shown as malformed.

Usage:
  canopenctl mpdo watch --cob-id <id> [flags]

Examples:
  canopenctl mpdo watch --cob-id 0x185 --cob-id 0x285 -i vcan0
  canopenctl mpdo watch --cob-id 0x285 --eds node5.eds --count 10 --format json -i can0

Flags:
      --cob-id stringArray   COB-ID of MPDOs to decode (hexadecimal; repeatable, comma-separated)
  -n, --count int            Stop after this many MPDOs (0: no limit)
      --duration duration    Stop after this time (0: until Ctrl-C)
      --eds string           EDS or DCF naming the objects
  -h, --help                 help for watch
  -i, --interface string     CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl boot

```text
boot runs the network boot-up of CiA 302-2 as an NMT master (go-canopen's
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
reports them); a slave that failed makes the exit code 1.

Usage:
  canopenctl boot --slave <node>[=<config>]... [flags]

Examples:
  canopenctl boot --slave 5=node5.dcf --slave 6=node6.cdcf -i vcan0
  canopenctl boot --lss --slave 5=node5.dcf --expect 5=F7A:C0C7:0:1001 --slave 6 --expect 6=F7A:C0C7:0:1002 -i can0 --yes

Flags:
      --boot-time duration    How long the boot-up waits for the mandatory slaves (0x1F89; 0: no limit) (default 10s)
      --brs                   With --fd: send CAN FD frames with bit rate switching
      --expect stringArray    Expected identity of a slave: NODE=vendor:product:revision:serial in hexadecimal, 0 not checked (repeatable)
      --fd                    CANopen FD (CiA 1301): open the interface for CAN FD and boot a CANopen FD network (USDO from --master-id, FD NMT and heartbeats)
  -h, --help                  help for boot
  -i, --interface string      CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --lss                   Give unconfigured devices the node-ID of the slave whose --expect identity they have (LSS Fastscan)
      --lss-window duration   Listening window of each Fastscan query of --lss (default 25ms)
      --master-id int         Node-ID of canopenctl's manager node (default 127)
      --max-time duration     Bound of the whole boot-up (default 1m0s)
      --no-start              Leave the slaves pre-operational
      --optional string       Slaves that are not mandatory: 5,7-9
      --retries int           Repeat a transfer that timed out up to this many times (0..10)
      --slave stringArray     A slave: NODE, or NODE=FILE with its DCF or concise DCF (repeatable)
      --store                 Make each configured slave save its parameters (0x1010:01)
      --timeout duration      SDO response timeout (default 1s)
      --yes                   Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl cdcf

```text
A concise DCF is the binary node configuration a CiA 302 manager keeps in
0x1F22 and writes to the node at boot-up: a count, then entries of index,
sub-index, size and data, written in their order. convert makes one from a
DCF (with the PDO parameters in the sequence CiA 301 requires), show lists
one, apply writes one (or a DCF) to a node.

Usage:
  canopenctl cdcf [command]

Available Commands:
  apply       Write a concise DCF (or a DCF) to a node, entry by entry (needs --yes off virtual interfaces)
  convert     Convert a DCF into a concise DCF
  show        List the entries of a concise DCF (or of the concise DCF of a DCF)

Flags:
  -h, --help   help for cdcf

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)

Use "canopenctl cdcf [command] --help" for more information about a command.
```

## canopenctl cdcf convert

```text
convert extracts the configuration of a DCF (every writable sub-object with a
ParameterValue, without the "refuse write on download" flag, $NODEID
evaluated for --node-id or the DCF's node-ID) as a concise DCF, PDO
parameters in the CiA 301 sequence (invalidate, clear the mapping, map,
enable, make valid). --output writes it to a file (the result describes
it); without it the binary goes to stdout.

Usage:
  canopenctl cdcf convert <dcf> [flags]

Examples:
  canopenctl cdcf convert node5.dcf -o node5.cdcf
  canopenctl cdcf convert device.eds --node-id 5 -o node5.cdcf

Flags:
  -h, --help            help for convert
      --node-id int     Node-ID for $NODEID (default: the DCF's)
  -o, --output string   Write the concise DCF to this file (default: stdout)

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl cdcf show

```text
show decodes a concise DCF and lists its entries in order (object, data);
--eds names the objects. Given a DCF it shows the concise DCF that convert
would write.

Usage:
  canopenctl cdcf show <file> [flags]

Examples:
  canopenctl cdcf show node5.cdcf --eds device.eds
  canopenctl cdcf show node5.dcf --format json

Flags:
      --eds string    EDS or DCF naming the objects
  -h, --help          help for show
      --node-id int   Node-ID for $NODEID of a DCF (default: the DCF's)

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl cdcf apply

```text
apply writes the entries of a concise DCF to a node one by one, in their order,
as a CiA 302 manager does (nothing is regrouped: the file must hold the
sequence the node needs; convert writes PDOs so). A DCF is converted first.
The first refused entry stops it (exit 11, the abort code and object in the
error). --dry-run lists the entries without sending anything.

Usage:
  canopenctl cdcf apply <node> <file> [flags]

Examples:
  canopenctl cdcf apply 5 node5.cdcf -i vcan0
  canopenctl cdcf apply 5 node5.dcf --dry-run

Flags:
      --brs                With --fd: send CAN FD frames with bit rate switching
      --client-id int      With --fd: node-ID of canopenctl's USDO client (its requests go out on 0x600 + it) (default 127)
      --dry-run            List the entries without sending anything
      --eds string         EDS or DCF naming the objects
      --fd                 CANopen FD (CiA 1301): open the interface for CAN FD and write over USDO instead of SDO
  -h, --help               help for apply
  -i, --interface string   CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --retries int        Repeat a transfer that timed out up to this many times (0..10)
      --timeout duration   SDO response timeout (default 1s)
      --yes                Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl store

```text
store writes the signature "save" to 0x1010 of a node: the node saves the
current values of a parameter group (--group all, communication,
application, manufacturer, or a sub-index) in non-volatile memory and loads
them at its next resets. A device that cannot save refuses (exit 11).

Usage:
  canopenctl store <node> [flags]

Examples:
  canopenctl store 5 -i vcan0
  canopenctl store 5 --group communication -i can0 --yes

Flags:
      --brs                With --fd: send CAN FD frames with bit rate switching
      --client-id int      With --fd: node-ID of canopenctl's USDO client (its requests go out on 0x600 + it) (default 127)
      --fd                 CANopen FD (CiA 1301): open the interface for CAN FD and write over USDO instead of SDO
      --group string       Parameter group: all, communication, application, manufacturer, or a sub-index (default "all")
  -h, --help               help for store
  -i, --interface string   CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --retries int        Repeat a transfer that timed out up to this many times (0..10)
      --timeout duration   SDO response timeout (default 1s)
      --yes                Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl restore

```text
restore writes the signature "load" to 0x1011 of a node: the node discards the
saved values of a parameter group (--group all, communication, application,
manufacturer, or a sub-index) and has its default values from its next
reset on. --reset sends that reset (NMT reset node) at once.

Usage:
  canopenctl restore <node> [flags]

Examples:
  canopenctl restore 5 --reset -i vcan0
  canopenctl restore 5 --group application -i can0 --yes

Flags:
      --brs                With --fd: send CAN FD frames with bit rate switching
      --client-id int      With --fd: node-ID of canopenctl's USDO client (its requests go out on 0x600 + it) (default 127)
      --fd                 CANopen FD (CiA 1301): open the interface for CAN FD and write over USDO instead of SDO
      --group string       Parameter group: all, communication, application, manufacturer, or a sub-index (default "all")
  -h, --help               help for restore
  -i, --interface string   CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --reset              Reset the node (NMT reset node) so that the defaults apply now
      --retries int        Repeat a transfer that timed out up to this many times (0..10)
      --timeout duration   SDO response timeout (default 1s)
      --yes                Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl program

```text
The program commands use the program download objects of a device: program
data 0x1F50 (the image), program control 0x1F51 (stop, start, reset,
clear; the state), application software identification 0x1F56 and flash
status 0x1F57. Program control commands are accepted in pre-operational.

Usage:
  canopenctl program [command]

Available Commands:
  control     Write a program control command to 0x1F51 (needs --yes off virtual interfaces)
  download    Replace the program of a node (needs --yes off virtual interfaces)
  status      Read the program state, identification and flash status of a node (SDO reads)

Flags:
  -h, --help   help for program

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)

Use "canopenctl program [command] --help" for more information about a command.
```

## canopenctl program download

```text
download replaces the program of a node as a CiA 302 manager does: stop the
program (0x1F51 = 0), clear it (3), write the image to 0x1F50 (SDO block
transfer, segmented when the device refuses block; USDO bulk with --fd),
poll the flash status 0x1F57 until programming ends, compare the
application software identification 0x1F56 with --identification (or the
CRC-32 of the file with --crc32), start the program (1) and wait until
0x1F51 reads started. The node must be pre-operational. A flash error or a
wrong identification is exit 12 (program_failed); --max-time bounds the
whole download, --timeout each SDO transfer.

Usage:
  canopenctl program download <node> <file> [flags]

Examples:
  canopenctl program download 5 firmware.bin --crc32 -i vcan0
  canopenctl program download 5 firmware.bin --identification 0x01020304 -i can0 --yes

Flags:
      --brs                     With --fd: send CAN FD frames with bit rate switching
      --client-id int           With --fd: node-ID of canopenctl's USDO client (its requests go out on 0x600 + it) (default 127)
      --crc32                   Expect the CRC-32 (IEEE) of the file as identification 0x1F56
      --fd                      CANopen FD (CiA 1301): open the interface for CAN FD and use USDO instead of SDO (bulk transfer of the image)
  -h, --help                    help for download
      --identification string   Expected application software identification 0x1F56 after the download
  -i, --interface string        CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --max-time duration       Bound of the whole download (default 5m0s)
      --poll duration           Polling interval of the flash status and program control (default 100ms)
      --program int             Program number: the sub-index of 0x1F50/0x1F51 (1..254) (default 1)
      --retries int             Repeat a transfer that timed out up to this many times (0..10)
      --timeout duration        SDO response timeout (default 1s)
      --yes                     Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl program control

```text
control writes a command to the program control 0x1F51 of a node: stop (0),
start (1), reset (2) or clear (3). The device accepts them in
pre-operational and refuses others with an abort (exit 11).

Usage:
  canopenctl program control <node> <stop|start|reset|clear> [flags]

Examples:
  canopenctl program control 5 stop -i vcan0

Flags:
      --brs                With --fd: send CAN FD frames with bit rate switching
      --client-id int      With --fd: node-ID of canopenctl's USDO client (its requests go out on 0x600 + it) (default 127)
      --fd                 CANopen FD (CiA 1301): open the interface for CAN FD and use USDO instead of SDO (bulk transfer of the image)
  -h, --help               help for control
  -i, --interface string   CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --program int        Program number: the sub-index of 0x1F50/0x1F51 (1..254) (default 1)
      --retries int        Repeat a transfer that timed out up to this many times (0..10)
      --timeout duration   SDO response timeout (default 1s)
      --yes                Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl program status

```text
status reads the program control 0x1F51 (the program's state), the
application software identification 0x1F56 and the flash status 0x1F57 of a
node. It only sends read requests.

Usage:
  canopenctl program status <node> [flags]

Examples:
  canopenctl program status 5 -i vcan0

Flags:
      --brs                With --fd: send CAN FD frames with bit rate switching
      --client-id int      With --fd: node-ID of canopenctl's USDO client (its requests go out on 0x600 + it) (default 127)
      --fd                 CANopen FD (CiA 1301): open the interface for CAN FD and use USDO instead of SDO (bulk transfer of the image)
  -h, --help               help for status
  -i, --interface string   CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --program int        Program number: the sub-index of 0x1F50/0x1F51 (1..254) (default 1)
      --retries int        Repeat a transfer that timed out up to this many times (0..10)
      --timeout duration   SDO response timeout (default 1s)

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl simulate

```text
simulate runs a complete CANopen device (go-canopen's node): NMT slave with
boot-up, SDO server, heartbeat producer and consumer, node guarding, EMCY,
SYNC, TIME and every PDO of its dictionary, built from --eds (an EDS or DCF;
objects a dictionary cannot hold are skipped with a warning) or, without it, a
built-in device with the CiA 301 objects and application objects 0x2000
(counter, UNSIGNED32, mapped in TPDO1, event-driven), 0x2001 (UNSIGNED8,
mapped in RPDO1), 0x2002 (VISIBLE_STRING) and 0x2003 (DOMAIN).

--drive adds a simulated CiA 402 drive (state machine, profile position and
velocity, homing...). --tick increments objects every --tick-period, so that
event-driven TPDOs carry changing data; --emcy raises an emergency
--emcy-after the start. The node runs until --duration or Ctrl-C and prints
its NMT state changes.

--lss makes the node an LSS slave (CiA 305): without --node-id it starts
unconfigured (node-ID 0xFF: silent until an LSS master such as "canopenctl
lss" gives it a node-ID); --serial sets its serial number 0x1018:04 so that
several simulated devices have distinct LSS addresses. --fd makes it a
CANopen FD device (CiA 1301): USDO server instead of SDO, FD PDOs, EMCY,
heartbeat. --store FILE keeps its parameters (0x1010/0x1011 and the LSS
node-ID) in FILE across restarts; --program adds the program download
objects 0x1F50..0x1F57 (CiA 302-3) with a program memory whose 0x1F56 is
the CRC-32 of the image. It transmits like any device: on an interface that is
not virtual it needs --yes. On "virtual" it shares the in-process bus with
nothing else; use vcan0 to talk to it from another canopenctl.

Usage:
  canopenctl simulate [flags]

Examples:
  canopenctl simulate -i vcan0 --node-id 5
  canopenctl simulate -i vcan0 --eds io401.eds --node-id 5 --tick 6000:01 --auto-start
  canopenctl simulate -i vcan0 --node-id 3 --drive
  canopenctl simulate -i vcan0 --lss --serial 0x1001 --store node.cdcf
  canopenctl simulate -i vcan0 --node-id 5 --fd --program

Flags:
      --auto-start              Enter operational right after the boot-up (and after every reset)
      --brs                     With --fd: send CAN FD frames with bit rate switching
      --drive                   Simulate a CiA 402 drive (adds its objects where missing)
      --drive-period duration   Simulation step of the drive (default 1ms)
      --duration duration       Stop after this time (0: until Ctrl-C)
      --eds string              EDS or DCF of the simulated device (default: a built-in device)
      --emcy string             Raise this emergency: CODE[:REG[:MSEF]] in hexadecimal (e.g. 2310, 5000:81:0102)
      --emcy-after duration     Delay of --emcy after the start
      --fd                      CANopen FD (CiA 1301): open the interface for CAN FD and run a CANopen FD device (USDO server, FD PDOs, EMCY, heartbeat)
      --heartbeat int           Producer heartbeat time 0x1017 in ms (default: the dictionary's) (default -1)
  -h, --help                    help for simulate
  -i, --interface string        CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --lss                     Make the node an LSS slave (CiA 305); without --node-id it starts unconfigured
      --node-id int             Node-ID 1..127 (default: the DCF's; with --lss: none, the node starts unconfigured)
      --program                 Add the program download objects 0x1F50..0x1F57 with an in-memory program
      --serial string           Serial number 0x1018:04 (part of the LSS address; default: the dictionary's)
      --store string            Keep the parameters (0x1010/0x1011, LSS store) in this file
      --tick stringArray        Increment this unsigned object (IIII:SS, hexadecimal) every --tick-period (repeatable)
      --tick-period duration    Period of --tick (default 100ms)
      --yes                     Confirm a state-changing transmission on an interface that is not virtual

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl version

```text
Print the version of canopenctl, go-canopen and go-canbus

Usage:
  canopenctl version [flags]

Flags:
  -h, --help   help for version

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl completion

```text
Generate a shell completion script

Usage:
  canopenctl completion bash|zsh|fish|powershell [flags]

Flags:
  -h, --help   help for completion

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```
