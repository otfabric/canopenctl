# Changelog

All notable changes are listed here; release notes are in
[RELEASE.md](RELEASE.md).

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
