# Security

## Reporting

Report vulnerabilities privately, not in public issues: email
**security@otfabric.io**, or open a private advisory through GitHub's
security advisories of otfabric/canopenctl ("Report a vulnerability"). Include
a description, steps to reproduce, the affected version (`canopenctl version`
output) and the impact you see.

## Supported versions

| Version | Supported |
|---------|-----------|
| Latest release | Yes |
| Older releases | No |

## Operational safety

canopenctl talks to CANopen devices that control physical processes:
drives, valves, heaters, power electronics. It is read-only by default:

- `monitor`, `decode`, `heartbeat watch`, `emcy watch`, `mpdo watch` and
  `pdo watch` with `--cob-id`/`--map` never transmit (slcan adapters are opened listen-only).
- `sdo read`, `sdo dump`, `scan`, `pdo show`, `pdo watch`, `dcf export`,
  `program status` and `cia402 state` send SDO (or USDO) read requests only. They change no device state
  by the standard, but a device may implement side effects on reads of
  vendor objects; `scan` addresses every node-ID.
- Every state-changing command — `sdo write`, `nmt`, `pdo map`, `dcf apply`,
  `cdcf apply`, `cia402 enable/disable/fault-reset/quick-stop/mode/move/velocity`,
  `sync`, `time`, every `lss` command, `mpdo configure/scanner/dispatcher`,
  `boot` (which resets every node), `store`, `restore`, `program
  download/control` (which replace or stop a device's program), `simulate` —
  requires `--yes` on any interface that is not
  virtual (vcan, vxcan or the in-process bus), and validates its input
  before the first frame.
- Productions are bounded (`--count`, `--duration`, `--timeout`,
  `--step-timeout`, `--max-time`).

A CANopen network has no authentication: an NMT command, a controlword write
or a SYNC from any node is obeyed. Test on a vcan interface or a test bench
first; know the network's NMT master before you send NMT commands (two
masters fight), and keep a hardware emergency stop at hand when operating
drives. `cia402 quick-stop` and `cia402 disable` are not safety functions.

Raw CAN sockets need no privilege on Linux: any local user can monitor and
transmit on CAN interfaces. Restrict access to the host accordingly; the
`--yes` confirmation guards against mistakes, not against malicious users.
`-i slcan:tcp://host:port` reaches a serial-to-TCP bridge over plain TCP,
without authentication or encryption: whoever reaches that port can read and
send on the bus. Keep such bridges on an isolated network.

## Untrusted input

EDS/DCF files and traces may come from untrusted sources: their parsers live
in go-canopen and go-canbus and are fuzzed there; canopenctl's own parsers
of values, PDO mappings, node lists, indices, data types and COB-IDs are
fuzzed in this repository (`make fuzz`). Generated Go code (`eds generate`)
embeds names from the EDS only as sanitized identifiers and quoted strings.
In text and table output, names and descriptions from EDS and DCF files and
interface names from traces are written with their control characters (C0,
DEL, C1, also as single Latin-1 bytes) and bidirectional controls escaped
(`\x1b`, `\u202e`); JSON output escapes them as JSON does, and device
values are quoted. `sdo read --output` and the `-o` flags write where they are told; they do not
follow names from files or devices.

## Limits

The bounds that apply to input and to long runs; a value beyond a flag's
range is refused with exit 3 before anything is transmitted.

| What | Bound | Beyond it |
|------|-------|-----------|
| EDS, DCF and concise DCF files (every `--eds`, `eds`, `dcf`, `cdcf`, `boot --slave`, `simulate --eds`) | 16 MiB (16,777,216 bytes), go-canopen's own bound | Refused (exit 3, `invalid_eds`): a larger regular file is not read, a pipe or device is read up to the bound + 1 byte. Each file is read once |
| Sub-objects of a dictionary built from an EDS or DCF | 65,536 (go-canopen) | Refused (exit 3, `invalid_eds`) |
| String and domain values a `CompactSubObj` array copies | 1 MiB per file (go-canopen) | The remaining elements are skipped with a warning |
| SDO and USDO uploads (`--max-size`) | 1..1,073,741,824 bytes; default 1,048,576 (go-canopen's `sdo.DefaultMaxSize`) | The upload is aborted (exit 6) |
| SDO and USDO `--timeout`, LSS `--timeout` | more than 0, at most 1 minute | Exit 3 |
| `--retries` (SDO, USDO and LSS) | 0..10 | Exit 3 |
| `scan --timeout`, `scan --parallel` | at most 10 s; 1..127 nodes at once | Exit 3 |
| Integer flags | -2,147,483,648..2,147,483,647 on every platform, then the flag's own range | Exit 3 (a value that is not a number, or does not fit 64 bits, exit 2) |
| Frames queued by the bus and by `monitor`, `decode` and `mpdo watch` | 65,536 frames each | Dropped and counted (`dropped` in the summary) |
| Events queued between go-canopen and the output of `heartbeat watch`, `emcy watch`, `pdo watch` | 1024 events | Dropped and counted (`dropped`, and a line on stderr in text output); `heartbeat watch` still ends with 1 for a lost node |
| State changes queued by `simulate` | 256 | Dropped and counted (`dropped`) |
| Productions (`sync`, `time`) | `--count` 0..1,000,000; `--count 0` needs `--duration`; `--period` at least 1 ms | Exit 3 |
| MPDO scanner and dispatcher lists | 254 entries, lists 1..48 | Exit 3 |
| PDO mappings (`pdo map`, `pdo watch --map`) | 64 entries, 64 bits | Exit 3 |
| `slcan:tcp://` connection | 5 s to connect | Exit 4 (`interface_unavailable`) |
| `program download` image, `sdo write --input` | Not bounded: the operator's own file, read whole and sent | — |

