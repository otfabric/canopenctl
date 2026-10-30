# Limitations

- **Interfaces.** SocketCAN (Linux), serial-line CAN adapters (slcan,
  LAWICEL ASCII) and the in-process virtual bus. Vendor adapters with their
  own drivers (PEAK PCAN-Basic, Kvaser CANlib, Vector XL, IXXAT) are not
  supported outside Linux, where their SocketCAN drivers work. slcan has no
  error frames and no hardware filters.
- **CANopen FD** (`--fd`, CiA 1301) is offered where a command reads or writes
  dictionaries, sends NMT/SYNC or receives heartbeats and emergencies, and by
  `simulate`. Not with `--fd`: `pdo watch` (FD PDOs; `monitor --fd` shows
  them undecoded as PDOs), `cia402` (SDO only), `dcf export` (SDO only),
  `time` (TIME is classical in CANopen FD too) and `lss` (classical frames).
  USDO transfers go to one node: the group and broadcast transfers of the
  USDO (`UploadMulti`/`DownloadMulti`) are not commands. The USDO
  segmented-transfer layout is go-canopen's interpretation of the public
  sources (see its usdo package): other stacks may differ. slcan adapters
  are opened without a data phase bit rate. `monitor` and `decode` skip
  error frames (counted in `frames`).
- **Predefined connection set.** `monitor` and `decode` interpret frames by
  their default identifiers (as Wireshark does); a device that moved a PDO or
  SDO channel elsewhere is decoded by the identifier's default meaning. The
  SDO commands use the default SDO channel (0x600/0x580 + node-ID); other
  channels (0x1200 sub 1/2, 0x1280..) are not selectable.
- **LSS** (`lss`) acts on one network with one LSS master: canopenctl does not
  detect another master. `lss activate` switches the devices' bit rate, not
  canopenctl's own interface. `lss assign-all` cannot see node-IDs already
  used by configured devices: give them with `--avoid`. Table selectors
  other than the CiA 301 table are not offered.
- **`boot`** runs go-canopen's CiA 302 manager for one boot-up and exits:
  it does not stay as the network's NMT master (no supervision of the
  slaves afterwards), sends the slaves no heartbeat consumer configuration
  of its own, and does not implement the flying master, the configuration
  file 0x1F20 or software updates from 0x1F58 (use `program download`).
  Optional slaves that do not boot within a second after the mandatory ones
  are reported as not booted.
- **`program`** writes the image as it is (no file format such as Intel
  HEX is parsed) to one program number; `--crc32` assumes the device
  reports the CRC-32 of the image as 0x1F56, as `simulate --program` does.
- **No node guarding master.** `heartbeat watch` follows heartbeats (and
  reads node guarding responses as heartbeats) but does not send guard
  requests.
- **SYNC and TIME timing** come from a Go ticker in a user-space process:
  jitter of tens to hundreds of microseconds, more under load. Use them for
  commissioning and tests, not as a precision clock master.
- **`scan`** reads 0x1000 and 0x1018 by SDO with a short timeout; a node
  slower than `--timeout` is missed. Nodes in Stopped do not answer SDO.
- **`dcf apply`** writes the DCF's values as they are, in the order of
  the file. go-canopen applies PDO parameters in the CiA 301 order and
  releases EMCY (0x1014), SYNC (0x1005) and TIME (0x1012) COB-IDs before
  changing their CAN-ID; other device-specific write rules are the
  device's (a refusal is exit 11).
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
- **Interop coverage** (canopen-interop v0.2.0, [TESTING.md](TESTING.md)).
  `store`/`restore` and `program` are tested only against go-canopen
  (in process, `simulate --store`/`--program`): no device of the
  canopen-interop contract has 0x1010/0x1011 or 0x1F50/0x1F51 (CANopenNode's
  device is built without storage). `--fd` is tested only against go-canopen
  (`simulate --fd`): no open-source reference stack implements CANopen FD.
  MPDOs are tested only with Lely (CANopenNode v4.1 and python canopen have
  none), and a scanner list written by `mpdo scanner` is verified by SDO,
  not by a SAM-MPDO of that device (Lely's device sends SAM-MPDOs only when
  its application asks). Lely is the only stack boot master and concise DCF
  generator; `cdcf convert` matches dcfgen for integer and REAL values
  (dcfgen takes no strings) and Lely's dictionary dump for one index range.
- **Stack behaviour seen in interop.** CANopenNode v4.1's LSS slave does
  not answer `lss identify` (with or without `--address`): use `lss
  fastscan`. A Lely LSS slave applies a node-ID from `lss configure` or
  `lss assign-all` only at its next NMT reset communication: send `nmt
  reset-comm all` afterwards (`boot --lss` does this as part of the
  boot-up). A Lely slave answers Fastscan also when it has a node-ID, so
  `lss fastscan` lists configured Lely devices too (`assign-all` copes).
