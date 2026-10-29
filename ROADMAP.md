# Roadmap

Planned, roughly in order:

1. **LSS** (CiA 305): `lss` commands to switch states, inquire identities,
   configure node-ID and bit rate, and fastscan for unconfigured nodes.
2. **More CiA 402**: profile torque, homing and cyclic synchronous modes as
   commands; `cia402 watch` (statusword and actual values from a TPDO).
3. **SDO channels**: `--cob-id` for SDO commands (additional server
   channels 0x1201.., SDO manager setups).
4. **Node guarding master**: `guard watch` sending guard requests.
5. **`pdo send`**: produce a node's RPDO from values (`master.RPDOSender`),
   bounded and behind `--yes`.
6. **Boot procedure**: `boot <node> --dcf` (CiA 302-style: identity check,
   configuration, start) and `store`/`restore` (0x1010/0x1011).
7. **Recording**: `monitor -o trace.pcapng` like canbusctl, and `decode`
   with `--eds` to name PDO values and SDO objects.
8. **CANopen FD** (CiA 1301) once go-canopen supports it.
