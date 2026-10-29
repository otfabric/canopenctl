# Limitations

- **Interfaces.** SocketCAN (Linux), serial-line CAN adapters (slcan,
  LAWICEL ASCII) and the in-process virtual bus. Vendor adapters with their
  own drivers (PEAK PCAN-Basic, Kvaser CANlib, Vector XL, IXXAT) are not
  supported outside Linux, where their SocketCAN drivers work. slcan has no
  error frames and no hardware filters.
- **Classical CAN only.** CANopen FD (CiA 1301) is not supported: PDOs carry
  at most 8 bytes, SDO is the classical protocol. `monitor` and `decode` skip
  CAN FD and error frames (counted in `frames`).
- **Predefined connection set.** `monitor` and `decode` interpret frames by
  their default identifiers (as Wireshark does); a device that moved a PDO or
  SDO channel elsewhere is decoded by the identifier's default meaning. The
  SDO commands use the default SDO channel (0x600/0x580 + node-ID); other
  channels (0x1200 sub 1/2, 0x1280..) are not selectable.
- **No LSS.** Layer setting services (CiA 305: node-ID and bit rate
  configuration, fastscan) are decoded by `monitor` but not offered as
  commands; `scan` finds configured nodes only.
- **No node guarding master.** `heartbeat watch` follows heartbeats (and
  reads node guarding responses as heartbeats) but does not send guard
  requests.
- **SYNC and TIME timing** come from a Go ticker in a user-space process:
  jitter of tens to hundreds of microseconds, more under load. Use them for
  commissioning and tests, not as a precision clock master.
- **`scan`** reads 0x1000 and 0x1018 by SDO with a short timeout; a node
  slower than `--timeout` is missed. Nodes in Stopped do not answer SDO.
- **`dcf apply`** writes the DCF's values as they are. go-canopen applies
  PDO parameters in the CiA 301 order, but other objects whose identifier
  cannot change while valid (EMCY 0x1014, SYNC 0x1005, TIME 0x1012) are
  written directly: a DCF made for another node-ID, with absolute COB-IDs,
  is refused by the device (exit 11). Use a DCF with `$NODEID` values or one
  made for the node (`eds convert --node-id`, `dcf export`).
- **`dcf export`** writes the dictionary as go-canopen models it: every
  object expanded (no compact storage), comments and vendor sections of the
  EDS not kept; the pre-defined error field 0x1003 keeps the EDS default.
- **`eds convert`** writes CiA 306 in a canonical layout: comments, key
  order and the spelling of numbers are not kept.
- **`cia402`** covers the state machine, modes of operation, profile position
  and profile velocity; profile torque, homing and the cyclic synchronous
  modes are not commands yet (the simulated drive supports them).
- **Virtual bus scope.** `-i virtual` is private to one canopenctl process:
  a `simulate -i virtual` cannot be reached by another process. Use vcan.
- **Platforms.** SocketCAN commands exit 8 on macOS and Windows.
