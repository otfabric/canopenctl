# Releases

Releases are made by the Release workflow (Actions → Release → Run workflow,
with the version bump). It computes the tag, builds the binaries for linux
(amd64, arm64, armv7), darwin (amd64, arm64) and windows (amd64, arm64) and
publishes the GitHub release with the section of this file that matches the
version as its notes. Add the section before running it.

## v0.2.1

**Date:** 2026-10-30
**Previous release:** v0.2.0

Built on go-canopen v0.3.1 (was v0.3.0).

- **`boot`**: go-canopen v0.3.1 fixes a race of the network boot-up. A
  slave whose boot-up arrived before the manager's NMT reset communication
  was already being configured when the reset undid its configuration, and
  `boot` reported it failed (status K); the boot-up now cancels the running
  boot slave processes before the reset.
- **Interop scenarios for the v0.2.0 commands** against canopen-interop
  v0.2.0 (contract 1.1, images pinned by digest):
  - `lss` against CANopenNode's and Lely's LSS slaves: identify, select,
    inquire, bit timing, configure node-ID, store, Fastscan of unconfigured
    and configured devices, assign-all; and the LSS masters of all three
    stacks against `simulate --lss` (inquire, configure node-ID, store kept
    across a restart with `--store`, Fastscan, assign-all).
  - `mpdo configure`, `dispatcher`, `scanner` on Lely devices, checked
    with Lely's DAM- and SAM-MPDOs and by SDO; `mpdo watch` of Lely's MPDOs.
  - `boot` of every stack's device (concise DCF, identity, start), with the
    other product's identity (error status M, exit 1), with `--lss` for the
    stacks' LSS slaves, and of a network with a device of every stack;
    Lely's boot master booting `simulate`.
  - `cdcf convert` byte for byte against Lely's dcfgen and dictionary dump;
    `cdcf apply` to every stack's device, read back.
  - `store`/`restore`, `program` and `--fd`: recorded skips — no device of
    the contract has 0x1010/0x1011 or 0x1F50/0x1F51 (probed: abort
    0x06020000), no stack implements CANopen FD.
- Four new known differences, observed on the bus: CANopenNode v4.1's LSS
  slave answers neither identify service; Lely's LSS slave applies a
  configured node-ID only at NMT reset communication and answers Fastscan
  also when configured. LIMITATIONS.md says how to work with them.

Verification:

- Interop on Linux 7.0 arm64 (vcan0), full suite twice with identical
  results, the new scenarios three more times: 152 pass, 5 known
  differences, 63 skips, 0 failures (before: 93 pass, 1 known, 17 skips).
  Per adapter: canopennode 48/2/23, lely 62/3/8, pythoncanopen 41/0/32,
  mixed network 1 pass.
- `make check` (fmt, staticcheck, golangci-lint, vet, cross, govulncheck,
  tests with the race detector, coverage 93.0 %); golangci-lint and go vet
  with the interop build tag.

---

## v0.2.0

**Date:** 2026-10-29
**Previous release:** v0.1.1

Built on go-canopen v0.3.0 (was v0.2.0): LSS, MPDO, CiA 302 network
management and CANopen FD.

- **`lss`**: an LSS master (CiA 305). `switch` (global), `select`
  (selective, by LSS address), `inquire` (address and node-ID),
  `configure` (node-ID, bit rate from the CiA 301 table, store), `activate`
  (bit timing switch), `identify` (non-configured devices, or an address
  range), `fastscan` (the addresses of every unconfigured device) and
  `assign-all` (a node-ID for each, with `--avoid` and `--store`). A
  refusal is exit 12 (`lss_refused`), no answer exit 5 (`no_response`).
- **`mpdo`**: `configure` makes a TPDO or RPDO an MPDO in source or
  destination address mode, `scanner` and `dispatcher` write the object
  scanner and dispatcher lists, `watch` decodes MPDOs (passive).
- **`boot`**: the CiA 302-2 network boot-up as a manager — slaves with their
  DCF or concise DCF, expected identities, mandatory and optional slaves,
  `--lss` for the LSS step that gives unconfigured devices the node-ID of
  the slave whose identity they have, `--store`, `--no-start`. The result
  lists each slave's CiA 302 boot status; a failed slave is exit 1.
- **`cdcf convert`, `show`, `apply`**: concise DCFs (0x1F22) from DCFs
  (PDOs in the CiA 301 sequence), listed, and written entry by entry.
- **`store`** and **`restore`** (0x1010/0x1011, `--group`, `restore
  --reset`); **`program download`** (CiA 302-3: stop, clear, image, flash
  status, identification, start), **`program control`** and **`program
  status`**.
- **CANopen FD**: `--fd` (and `--brs`) on `sdo read/write/dump` (USDO
  client, `--block` for bulk downloads, `--client-id`), `scan`, `nmt`,
  `heartbeat watch`, `emcy watch` (20-byte FD emergencies), `sync`, `pdo
  show/map`, `dcf apply` and the new commands. `monitor --fd` and `decode`
  now decode CAN FD frames: USDO requests and responses (`--service usdo`)
  and FD emergencies; before, they skipped CAN FD frames. A USDO abort is
  exit 11 with its SDO equivalent and `usdoCode`.
- **`simulate`**: `--lss` (an LSS slave; without `--node-id` it starts
  unconfigured), `--serial`, `--fd`, `--store FILE` (store and restore,
  LSS store) and `--program` (program download objects with an in-memory
  program).
- JSON: new result and record shapes in the schemas (additions only,
  `schemaVersion` 1.0); new error codes `lss_refused`, `lss_conflict`,
  `no_node_id`, `program_failed`, `boot_failed`, `mpdo_inactive`.

Verification:

- In-process tests of every new command against go-canopen nodes on the
  virtual bus with the race detector: LSS slaves (unconfigured and
  configured, with and without a store), MPDO producer and consumer,
  store/restore and program download devices, the CiA 302 boot-up with
  and without the LSS step, CANopen FD nodes over USDO; JSON output
  validated against the schemas; whole-module coverage ≥ 90 %.
- End to end on vcan0 (Linux 7.0, CAN FD MTU): two unconfigured
  `simulate --lss` processes found by `lss fastscan` and booted by `boot
  --lss`; a `simulate --fd` process operated over USDO (`sdo read`, `nmt
  --wait`, `program download`, `scan`) while `monitor --fd` decoded the
  USDO traffic.
- Interop: the existing suite against CANopenNode, Lely CANopen and
  python canopen passes on the canopen-interop v0.2.0 images (pinned by
  digest; contract 1.1 only adds things) in CI on amd64 and arm64. The
  new commands have no interop scenarios yet.

---

## v0.1.1

**Date:** 2026-10-29
**Previous release:** v0.1.0

Built on go-canopen v0.2.0 (was v0.1.0).

- `sdo read` and `sdo write` with `--block` now handle empty values. A block
  transfer of 0 bytes may end without a data segment: Lely's SDO server sends
  such an upload, python canopen's client such a download. go-canopen v0.1.0
  read that end message as an out-of-order segment and timed out; v0.2.0
  accepts it.
- The interop suite pins the canopen-interop v0.1.0 images by digest.

---

## v0.1.0

**Date:** 2026-10-29
**Previous release:** none

## Summary

First release of canopenctl, a command-line toolkit for CANopen networks
built on go-canopen and go-canbus v0.7.0: monitor and decode traffic, SDO,
NMT, heartbeat, EMCY, SYNC, TIME, PDOs, EDS/DCF files, CiA 402 drives and
simulated nodes, on SocketCAN, serial-line CAN adapters and an in-process
virtual bus.

## Additions

- **`monitor`** decodes live traffic as CANopen messages (NMT, SYNC, TIME,
  EMCY with the meaning of its code, PDOs, SDO with segmented and block
  transfers attributed to their object, heartbeat, boot-up, node guarding,
  LSS), filtered by `--node` and `--service`; **`decode`** does the same for
  trace files (candump, JSON Lines, pcap, pcapng, ASC, BLF, TRC).
- **`sdo read`, `sdo write`, `sdo dump`**: expedited, segmented and block
  transfer, data types from `--type` or `--eds`, files for DOMAIN objects,
  `--verify`; an abort code is reported with its meaning and as exit code
  11 (JSON: the error's `abort` object).
- **`nmt`** to one node or all, with `--wait` for the heartbeat or boot-up
  that confirms it; **`heartbeat watch`** (boot-ups, state changes,
  timeouts); **`scan`** (device type, profile and identity of nodes 1..127).
- **`emcy watch`**, **`sync`** (period, count, counter) and **`time`**.
- **`pdo show`**, **`pdo map`** (CiA 301 configuration sequence, checked
  against `--eds`) and **`pdo watch`** (decoded values; passive with
  `--cob-id`/`--map`).
- **`eds validate`**, **`eds show`**, **`eds convert`** (EDS ↔ DCF with the
  node-ID), **`eds generate`** (typed Go code), **`dcf apply`** (with
  `--dry-run`) and **`dcf export`** (a node's values into a DCF).
- **`cia402 state`, `enable`, `disable`, `fault-reset`, `quick-stop`,
  `mode`, `move`, `velocity`**: a drive operated through its statusword,
  each step verified; a drive fault is exit 12 with the drive's state.
- **`simulate`**: a complete go-canopen node from an EDS or DCF (or a
  built-in device), optionally a CiA 402 drive, with ticking objects and an
  emergency on request, for demos and tests.
- **Safety**: every state-changing command needs `--yes` on an interface
  that is not virtual; passive commands open slcan adapters listen-only;
  productions are bounded.
- `--format text|table|json` with JSON Schemas (`schemas/`), JSON Lines
  streams, stable exit codes (`docs/exitcodes.md`).

## Verification

- In-process tests of every command against go-canopen nodes (CiA 401 I/O
  module, built-in device, CiA 402 drive) on the virtual bus, with the race
  detector; golden JSON validated against the schemas; ≥ 90 % coverage.
- End to end on vcan0 (Linux 7.0): simulated node and drive processes
  operated by canopenctl processes.
- Interop on vcan0 against CANopenNode v4.1, Lely CANopen 2.3.5 and python
  canopen 2.4.1 (canopen-interop): canopenctl against their devices, their
  clients against `canopenctl simulate`, their EDS checkers against
  `eds validate`.

---
