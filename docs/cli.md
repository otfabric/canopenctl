# canopenctl command reference

Generated from `canopenctl <command> --help` by scripts/gen-cli-doc.sh; do not edit.
Exit codes: [exitcodes.md](exitcodes.md). JSON output: [json-output.md](json-output.md).

```text
canopenctl is a command-line toolkit for CANopen (CiA 301, CiA 306 EDS/DCF,
CiA 402 drives), built on github.com/otfabric/go-canopen and go-canbus.

Read-only by default: monitor, decode, heartbeat watch, emcy watch and pdo
watch never transmit; sdo read, sdo dump, scan, pdo show, dcf export and
cia402 state only send SDO read requests. Commands that change a device or
the network (sdo write, nmt, pdo map, dcf apply, cia402 enable/disable/
fault-reset/quick-stop/mode/move/velocity, sync, time, simulate) need --yes on any
interface that is not virtual (vcan, vxcan or the in-process bus
"virtual"), and are bounded (--count, --duration, --timeout).

Interfaces: a SocketCAN interface (can0, vcan0; Linux), a serial-line CAN
adapter as slcan:<device> (slcan:/dev/ttyACM0, slcan:COM3) or
slcan:tcp://host:port, opened at --slcan-bitrate (passive commands open it
listen-only), or "virtual", an in-process bus for tests and simulation.

Results go to stdout as text (default), a table or JSON (--format); progress
and diagnostics go to stderr. Exit codes: docs/exitcodes.md.

Usage:
  canopenctl [command]

Available Commands:
  cia402      Operate a CiA 402 drive over SDO: state, enable, disable, mode, move, velocity
  completion  Generate a shell completion script
  dcf         Apply a DCF to a node, or export a node's configuration as a DCF
  decode      Decode a trace file as CANopen messages
  eds         Validate, show, convert and generate code from EDS and DCF files (CiA 306)
  emcy        Watch emergency messages
  heartbeat   Watch the heartbeats of the network
  help        Help about any command
  monitor     Decode live traffic as CANopen messages (passive)
  nmt         Send an NMT command to a node or to all nodes (needs --yes off virtual interfaces)
  pdo         Show, map and watch a node's process data objects
  scan        Find the nodes of a network: device type and identity of each (SDO reads)
  sdo         Read and write a node's object dictionary (SDO client)
  simulate    Run a simulated CANopen node (from an EDS, optionally a CiA 402 drive)
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
heartbeat, boot-up, node guarding and LSS. It never transmits; slcan adapters
are opened listen-only. --node and --service select messages; JSON output is
JSON Lines, one record per message and a summary at the end.

Usage:
  canopenctl monitor [flags]

Examples:
  canopenctl monitor -i can0
  canopenctl monitor -i vcan0 --node 5 --service sdo,emcy --format json

Flags:
  -n, --count int             Stop after this many messages shown (0: no limit)
      --duration duration     Stop after this time (0: until Ctrl-C)
  -h, --help                  help for monitor
  -i, --interface string      CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --node string           Only messages of (or addressed to) these nodes: 5, 1-10,20
      --service stringArray   Only these services (repeatable, comma-separated): emcy, heartbeat, lss, nmt, pdo, rpdo, sdo, sync, time, tpdo, unknown

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl decode

```text
decode reads a trace (candump log, JSON Lines, pcap, pcapng, ASC, BLF or PEAK
TRC; the format is detected from the content; "-" reads stdin) and decodes its
frames as CANopen messages, as monitor does. Malformed CANopen messages (a
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
      --service stringArray   Only these services (repeatable, comma-separated): emcy, heartbeat, lss, nmt, pdo, rpdo, sdo, sync, time, tpdo, unknown

Global Flags:
      --format string       Output format: text, json or table (env CANOPENCTL_FORMAT) (default "text")
      --slcan-baud int      Serial baud rate of an slcan adapter (USB adapters ignore it) (default 115200)
      --slcan-bitrate int   Bit rate an slcan:<device> adapter is opened with (0: keep the adapter's) (default 500000)
```

## canopenctl sdo

```text
The sdo commands are an SDO client (CiA 301): expedited, segmented and block
transfer (--block). A device that refuses a transfer answers with an SDO abort
code: it is printed with its meaning and ends the command with exit code 11
(JSON: the error's "abort" object). Indices are hexadecimal (1018, 0x1018);
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
      --eds string         EDS or DCF of the node: names and data types
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
      --eds string         EDS or DCF of the node: data types and names
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
      --eds string         EDS or DCF of the node (required)
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
  -n, --count int            Stop after this many events (0: no limit)
      --duration duration    Stop after this time (0: until Ctrl-C)
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
  -n, --count int           Stop after this many messages (0: no limit)
      --duration duration   Stop after this time (0: until Ctrl-C)
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
      --cob-id string       COB-ID of the SYNC message (hexadecimal) (default "80")
  -n, --count int           Send this many messages (0: until --duration) (default 10)
      --counter int         SYNC counter overflow value: 0 (no counter) or 2..240
      --duration duration   Stop after this time (0: no limit; needed with --count 0)
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
      --eds string         EDS or DCF of the node: names of the mapped objects
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
      --cob-id string      CAN identifier of the PDO (hexadecimal; default: keep the node's)
      --disable            Leave the PDO invalid (COB-ID bit 31)
      --eds string         EDS or DCF of the node: check the mapping before sending
      --event-timer int    Event timer (TPDO) or deadline (RPDO) in ms (default: keep the node's)
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
      --dry-run            List the entries without sending anything
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
its NMT state changes. It transmits like any device: on an interface that is
not virtual it needs --yes. On "virtual" it shares the in-process bus with
nothing else; use vcan0 to talk to it from another canopenctl.

Usage:
  canopenctl simulate [flags]

Examples:
  canopenctl simulate -i vcan0 --node-id 5
  canopenctl simulate -i vcan0 --eds io401.eds --node-id 5 --tick 6000:01 --auto-start
  canopenctl simulate -i vcan0 --node-id 3 --drive

Flags:
      --auto-start              Enter operational right after the boot-up (and after every reset)
      --drive                   Simulate a CiA 402 drive (adds its objects where missing)
      --drive-period duration   Simulation step of the drive (default 1ms)
      --duration duration       Stop after this time (0: until Ctrl-C)
      --eds string              EDS or DCF of the simulated device (default: a built-in device)
      --emcy string             Raise this emergency: CODE[:REG[:MSEF]] in hexadecimal (e.g. 2310, 5000:81:0102)
      --emcy-after duration     Delay of --emcy after the start
      --heartbeat int           Producer heartbeat time 0x1017 in ms (default: the dictionary's) (default -1)
  -h, --help                    help for simulate
  -i, --interface string        CAN interface: can0, vcan0, slcan:<device>, slcan:tcp://host:port or virtual (env CANOPENCTL_INTERFACE)
      --node-id int             Node-ID 1..127 (default: the DCF's)
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
