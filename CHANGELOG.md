# Changelog

All notable changes are listed here; release notes are in
[RELEASE.md](RELEASE.md).

## v0.2.1 (2026-10-30)

- go-canopen v0.3.1: the network boot-up of `boot` cancels the boot slave
  processes that run when it starts, before its NMT reset communication (a
  race: a slave whose boot-up arrived before the reset was being configured
  when the reset undid its configuration, and `boot` reported it failed
  with status K).
- Interop scenarios for the v0.2.0 commands against canopen-interop
  v0.2.0 (contract 1.1): `lss` against CANopenNode's and Lely's LSS slaves
  and every stack's LSS master against `simulate --lss`; `mpdo` with Lely's
  MPDO devices; `boot` of every stack's device, of a mixed network and with
  `--lss`, and Lely's boot master booting `simulate`; `cdcf convert` byte
  for byte against Lely's generators and `cdcf apply` to every device;
  store/restore, program and `--fd` recorded as skips where no stack has
  them. Four new known differences (LSS behaviour of CANopenNode v4.1 and
  Lely), observed on the bus.

## v0.2.0 (2026-10-29)

On go-canopen v0.3.0: LSS, MPDO, CiA 302 and CANopen FD.

- `lss switch/select/inquire/configure/activate/identify/fastscan/assign-all`:
  an LSS master (CiA 305) — node-IDs and bit rates by LSS address, Fastscan
  of unconfigured devices, node-IDs for all of them.
- `mpdo configure/scanner/dispatcher/watch`: multiplexed PDOs (DAM, SAM),
  object scanner and dispatcher lists, decoded MPDO traffic.
- `boot`: CiA 302 network boot-up as a manager (identity checks, concise
  DCF configuration, start), optionally with the LSS step.
- `cdcf convert/show/apply`: concise DCFs (0x1F22); `store`/`restore`
  (0x1010/0x1011); `program download/control/status` (CiA 302-3).
- `--fd` (and `--brs`): CANopen FD (CiA 1301) for `sdo read/write/dump`
  (USDO), `scan`, `nmt`, `heartbeat watch`, `emcy watch`, `sync`, `pdo
  show/map`, `dcf apply` and the new commands; `monitor --fd` and `decode`
  decode USDO and FD emergencies (they skipped CAN FD frames before).
- `simulate --lss` (unconfigured LSS slave), `--serial`, `--fd`, `--store`,
  `--program`.
- New error codes (exit 12): `lss_refused`, `lss_conflict`, `no_node_id`,
  `program_failed`, `boot_failed`, `mpdo_inactive`; USDO aborts carry
  `usdoCode`. Schemas extended (additions only).

## v0.1.1 (2026-10-29)

- go-canopen v0.2.0: SDO block transfers of empty values no longer time out.

## v0.1.0 (2026-10-29)

First release: a CANopen command-line toolkit on go-canopen and go-canbus
v0.7.0, on SocketCAN, slcan adapters and the in-process virtual bus.
`monitor` and `decode` (CANopen dissection with SDO transfer tracking,
node and service filters), `sdo read/write/dump` (expedited, segmented,
block; EDS names and types; abort codes as exit 11), `nmt` (with `--wait`),
`heartbeat watch`, `scan`, `emcy watch`, `sync`, `time`, `pdo
show/map/watch`, `eds validate/show/convert/generate`, `dcf apply/export`,
`cia402 state/enable/disable/fault-reset/quick-stop/mode/move/velocity` and
`simulate` (a go-canopen node from an EDS or built in, optionally a CiA 402
drive). `--yes` guards every state-changing transmission off virtual
interfaces; text, table and versioned JSON output with JSON Schemas; stable
exit codes. Tested in process against simulated nodes, end to end on vcan
and against CANopenNode, Lely CANopen and python canopen.
