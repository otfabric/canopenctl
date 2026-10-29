# Roadmap

Planned, roughly in order:

1. **Interop scenarios** for the v0.2.0 commands: LSS against CANopenNode's
   and Lely's LSS slaves, MPDO, store/restore and program download against
   their devices, CANopen FD where an image supports it.
2. **More CiA 402**: profile torque, homing and cyclic synchronous modes as
   commands; `cia402 watch` (statusword and actual values from a TPDO).
3. **SDO channels**: `--cob-id` for SDO commands (additional server
   channels 0x1201.., SDO manager setups).
4. **Node guarding master**: `guard watch` sending guard requests.
5. **`pdo send`**: produce a node's RPDO from values (`master.RPDOSender`),
   bounded and behind `--yes`.
6. **USDO group transfers**: `sdo read/write --fd` to a group or all nodes
   (`usdo.UploadMulti`/`DownloadMulti`); `--fd` for `pdo watch` and
   `cia402`.
7. **Recording**: `monitor -o trace.pcapng` like canbusctl, and `decode`
   with `--eds` to name PDO values and SDO objects.
8. **A resident manager**: `boot --stay` keeping the CiA 302 manager running
   (supervision, re-boot of lost slaves).
