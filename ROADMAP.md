# Roadmap

Planned, roughly in order:

1. **More CiA 402**: profile torque, homing and cyclic synchronous modes as
   commands; `cia402 watch` (statusword and actual values from a TPDO).
2. **SDO channels**: `--cob-id` for SDO commands (additional server
   channels 0x1201.., SDO manager setups).
3. **Node guarding master**: `guard watch` sending guard requests.
4. **`pdo send`**: produce a node's RPDO from values (`master.RPDOSender`),
   bounded and behind `--yes`.
5. **USDO group transfers**: `sdo read/write --fd` to a group or all nodes
   (`usdo.UploadMulti`/`DownloadMulti`); `--fd` for `pdo watch` and
   `cia402`.
6. **Recording**: `monitor -o trace.pcapng` like canbusctl, and `decode`
   with `--eds` to name PDO values and SDO objects.
7. **A resident manager**: `boot --stay` keeping the CiA 302 manager running
   (supervision, re-boot of lost slaves).
8. **Interop for store/restore, program and CANopen FD** once a
   canopen-interop device has 0x1010/0x1011 or 0x1F50/0x1F51, or a stack
   implements CiA 1301.
