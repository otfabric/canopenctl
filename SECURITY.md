# Security

## Reporting

Report vulnerabilities privately through GitHub's security advisories of
otfabric/canopenctl ("Report a vulnerability"), not in public issues.

## Operational safety

canopenctl talks to CANopen devices that control physical processes:
drives, valves, heaters, power electronics. It is read-only by default:

- `monitor`, `decode`, `heartbeat watch`, `emcy watch` and `pdo watch` with
  `--cob-id`/`--map` never transmit (slcan adapters are opened listen-only).
- `sdo read`, `sdo dump`, `scan`, `pdo show`, `pdo watch`, `dcf export` and
  `cia402 state` send SDO read requests only. They change no device state
  by the standard, but a device may implement side effects on reads of
  vendor objects; `scan` addresses every node-ID.
- Every state-changing command — `sdo write`, `nmt`, `pdo map`, `dcf apply`,
  `cia402 enable/disable/fault-reset/quick-stop/mode/move/velocity`,
  `sync`, `time`, `simulate` — requires `--yes` on any interface that is not
  virtual (vcan, vxcan or the in-process bus), and validates its input
  before the first frame.
- Productions are bounded (`--count`, `--duration`, `--timeout`,
  `--step-timeout`).

A CANopen network has no authentication: an NMT command, a controlword write
or a SYNC from any node is obeyed. Test on a vcan interface or a test bench
first; know the network's NMT master before you send NMT commands (two
masters fight), and keep a hardware emergency stop at hand when operating
drives. `cia402 quick-stop` and `cia402 disable` are not safety functions.

Raw CAN sockets need no privilege on Linux: any local user can monitor and
transmit on CAN interfaces. Restrict access to the host accordingly; the
`--yes` confirmation guards against mistakes, not against malicious users.

## Untrusted input

EDS/DCF files and traces may come from untrusted sources: their parsers live
in go-canopen and go-canbus and are fuzzed there; canopenctl's own parsers
of values, PDO mappings, node lists, indices, data types and COB-IDs are
fuzzed in this repository (`make fuzz`). Generated Go code (`eds generate`)
embeds names from the EDS only as sanitized identifiers and quoted strings.
`sdo read --output` and the `-o` flags write where they are told; they do not
follow names from files or devices.
