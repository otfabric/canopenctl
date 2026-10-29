# Releases

Releases are made by the Release workflow (Actions → Release → Run workflow,
with the version bump). It computes the tag, builds the binaries for linux
(amd64, arm64, armv7), darwin (amd64, arm64) and windows (amd64, arm64) and
publishes the GitHub release with the section of this file that matches the
version as its notes. Add the section before running it.

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
