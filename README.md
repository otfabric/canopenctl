# canopenctl

[![CI](https://github.com/otfabric/canopenctl/actions/workflows/ci.yml/badge.svg)](https://github.com/otfabric/canopenctl/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/otfabric/canopenctl/branch/main/graph/badge.svg)](https://codecov.io/gh/otfabric/canopenctl)

A command-line toolkit for CANopen networks: monitor and decode traffic,
read and write object dictionaries over SDO (and USDO, CANopen FD), manage
nodes with NMT, give devices node-IDs with LSS, boot a network as a CiA 302
manager, watch heartbeats and emergencies, configure and watch PDOs and
MPDOs, validate and convert EDS/DCF files and concise DCFs, download
programs, operate CiA 402 drives and simulate complete CANopen devices.
Built on [go-canopen](https://github.com/otfabric/go-canopen) and
[go-canbus](https://github.com/otfabric/go-canbus); pure Go, one static binary.

```sh
canopenctl monitor -i can0                                    # decode live traffic, passively
canopenctl scan -i can0                                       # nodes 1..127: device type and identity
canopenctl sdo read 5 0x1018 1 -i can0                        # vendor-ID of node 5
canopenctl sdo write 5 0x1017 0 1000 --type u16 -i can0 --yes # heartbeat every second
canopenctl sdo dump 5 --eds device.eds -i can0 --format table # every readable object, decoded
canopenctl nmt start all -i can0 --yes
canopenctl heartbeat watch -i can0 --supervise 1500ms         # boot-ups, state changes, losses
canopenctl emcy watch -i can0                                 # emergencies with their meaning
canopenctl pdo show 5 -i can0 --eds device.eds
canopenctl pdo map 5 --tpdo 1 --map 0x6041:00:16,0x6064:00:32 --type 255 --event-timer 100 -i can0 --yes
canopenctl pdo watch 5 --tpdo 1 --eds device.eds -i can0      # decoded process data
canopenctl eds validate device.eds
canopenctl eds convert device.eds --node-id 5 -o node5.dcf
canopenctl dcf apply 5 node5.dcf -i can0 --yes
canopenctl cia402 enable 3 -i can0 --yes && canopenctl cia402 move 3 10000 --wait -i can0 --yes
canopenctl simulate -i vcan0 --eds device.eds --node-id 5     # a complete CANopen device
canopenctl decode trip.log --node 5                           # candump, pcap(ng), BLF, ASC, TRC, JSON Lines
canopenctl lss fastscan -i can0 --yes                         # LSS addresses of devices without node-ID
canopenctl lss assign-all --first 10 --store -i can0 --yes    # give each of them a node-ID
canopenctl boot --slave 5=node5.dcf --slave 6 -i can0 --yes   # CiA 302 network boot-up
canopenctl cdcf convert node5.dcf -o node5.cdcf               # DCF -> concise DCF (0x1F22)
canopenctl store 5 -i can0 --yes                              # 0x1010 "save"
canopenctl program download 5 firmware.bin --crc32 -i can0 --yes
canopenctl mpdo configure 5 --tpdo 2 --mode sam --cob-id 285 -i can0 --yes
canopenctl sdo read 5 0x1018 1 --fd -i can0                   # CANopen FD: USDO
```

## Commands

| Group | Commands | Transmits? |
|-------|----------|-----------|
| Traffic | `monitor` (live), `decode` (trace files) | never |
| SDO | `sdo read`, `sdo dump` | SDO read requests only |
| | `sdo write` | yes: `--yes` off virtual interfaces |
| NMT | `nmt start\|stop\|preop\|reset-node\|reset-comm <node\|all>` | yes |
| | `heartbeat watch` | never |
| | `scan` | SDO read requests only |
| EMCY, SYNC, TIME | `emcy watch` | never |
| | `sync`, `time` | yes |
| PDO | `pdo show` | SDO read requests only |
| | `pdo map` | yes |
| | `pdo watch` | SDO reads of the configuration (none with `--cob-id`/`--map`) |
| EDS/DCF | `eds validate`, `show`, `convert` (EDS ↔ DCF), `generate` (Go code) | never (no interface) |
| | `dcf export` | SDO read requests only |
| | `dcf apply` | yes (`--dry-run` lists the entries) |
| CiA 402 | `cia402 state` | SDO read requests only |
| | `cia402 enable`, `disable`, `fault-reset`, `quick-stop`, `mode`, `move`, `velocity` | yes |
| LSS (CiA 305) | `lss switch`, `select`, `inquire`, `configure`, `activate`, `identify`, `fastscan`, `assign-all` | yes |
| MPDO | `mpdo configure`, `scanner`, `dispatcher` | yes |
| | `mpdo watch` | never |
| CiA 302 | `boot` (network boot-up, optionally with the LSS step) | yes |
| | `cdcf convert`, `show` | never (no interface) |
| | `cdcf apply`, `store`, `restore`, `program download`, `program control` | yes (`cdcf apply --dry-run` lists the entries) |
| | `program status` | SDO read requests only |
| Simulation | `simulate` (a node from an EDS or built in, optionally a CiA 402 drive, an LSS slave, a CANopen FD device, with store and program download) | yes |
| Other | `version`, `completion` | |

Full reference: [docs/cli.md](docs/cli.md). Output: `--format text` (default),
`table`, or versioned `json` ([docs/json-output.md](docs/json-output.md),
[schemas/](schemas)); streams are JSON Lines. Exit codes:
[docs/exitcodes.md](docs/exitcodes.md) — an SDO abort from a device is exit
11 with the abort code and its meaning, a drive fault exit 12.

Values on the command line: node-IDs 1..127 (decimal or 0x), indices
hexadecimal (`1018`, `0x1018`), sub-indices decimal or 0x hexadecimal, data
types as CiA 301 names (`UNSIGNED32`) or short (`u32`, `i16`, `f32`, `bool`,
`string`, `octets`, `domain`), PDO mappings as `IIII:SS:BITS`, LSS addresses
as `vendor:product:revision:serial` in hexadecimal, MPDO list entries as
`IIII:SS[+n]` (scanner) and `NODE:IIII:SS[+n]=IIII:SS` (dispatcher). `--eds`
gives names and data types wherever a command addresses objects.

## CANopen FD

`--fd` (with `--brs` for bit rate switching) makes a command speak CANopen FD
(CiA 1301): it opens the interface for CAN FD (a SocketCAN interface needs
the CAN FD MTU, `ip link set can0 mtu 72`) and reads and writes dictionaries
over the USDO instead of the SDO (`--client-id` is canopenctl's USDO
node-ID, default 127). It is offered by `sdo read/write/dump`, `scan`, `nmt`,
`heartbeat watch`, `emcy watch` (20-byte FD emergencies), `sync`, `pdo
show/map`, `dcf apply`, `cdcf apply`, `mpdo configure/scanner/dispatcher`,
`store`, `restore`, `program`, `boot` and `simulate`; `monitor --fd` decodes
USDO and FD emergencies, and `decode` decodes CAN FD frames of traces.

## Safety

CANopen devices drive motors, valves and power electronics. canopenctl is
**read-only by default** and conservative when it is not:

- `monitor`, `decode`, `heartbeat watch`, `emcy watch`, `mpdo watch` and
  `pdo watch` with `--cob-id`/`--map` never transmit; on a serial-line CAN adapter they open it
  listen-only, so the adapter does not even acknowledge frames.
- `sdo read`, `sdo dump`, `scan`, `pdo show`, `pdo watch`, `dcf export`,
  `program status` and `cia402 state` only send SDO read (upload) requests: they change no device
  state and need no confirmation. (A device may still act on a read through
  vendor-specific hooks; scan a production network with care.)
- Every command that changes a device or the network — `sdo write`, `nmt`,
  `pdo map`, `dcf apply`, `cdcf apply`, the `cia402` motion and state
  commands, `sync`, `time`, every `lss` command (they change the LSS state of
  devices), `mpdo configure/scanner/dispatcher`, `boot`, `store`, `restore`,
  `program download/control` and `simulate` — refuses to run on an interface that is not
  virtual (not `vcan`, `vxcan` or the in-process bus `virtual`) unless
  `--yes` is given, exit 9. The kind is read from the kernel (rtnetlink); a
  serial-line adapter (`slcan:`) always needs `--yes`. Everything is
  validated before the first frame: an invocation that exits 2, 3, 8 or 9
  has sent nothing.
- Transmissions are bounded: `sync` and `time` by `--count` (default 10 and
  1) or `--duration` (production without either is refused), SDO transfers
  by `--timeout` and `--retries`, `cia402` operations by `--step-timeout`,
  `scan` by `--timeout` and `--parallel`, LSS services by `--timeout`,
  `--retries` and `--window`, `boot` and `program download` by `--max-time`.
- Nothing probes the bus on its own: `scan`, `lss identify`, `lss fastscan`,
  `lss assign-all` and `boot` (which resets every node and, with `--lss`,
  runs a Fastscan) are the only commands that address nodes you did not
  name.

See [SECURITY.md](SECURITY.md).

## Interfaces and platforms

| | Linux amd64, arm64, armv7 | macOS amd64, arm64 | Windows amd64, arm64 |
|-|:-:|:-:|:-:|
| SocketCAN (`-i can0`, `-i vcan0`) | ✓ | exit 8 | exit 8 |
| Serial-line CAN adapters (`-i slcan:/dev/ttyACM0`, `slcan:COM3`, `slcan:tcp://host:port`) | ✓ | ✓ | ✓ |
| In-process virtual bus (`-i virtual`) | ✓ | ✓ | ✓ |
| `eds`, `decode`, `dcf apply --dry-run` | ✓ | ✓ | ✓ |

`--interface` (`-i`) defaults to `$CANOPENCTL_INTERFACE`. slcan adapters (CANable, CANtact, USBtin, LAWICEL)
are opened at `--slcan-bitrate` (default 500000; 0 keeps the adapter's) and
`--slcan-baud`. The virtual bus lives inside one canopenctl process: it is
what the tests and `simulate` demos use; to talk to a simulated node from
another canopenctl process, run both on `vcan0`:

```sh
sudo ip link add vcan0 type vcan && sudo ip link set vcan0 up
canopenctl simulate -i vcan0 --node-id 5 --tick 2000:00 &      # built-in device, TPDO1 = counter 0x2000
canopenctl scan -i vcan0
canopenctl nmt start 5 -i vcan0 --wait
canopenctl pdo watch 5 --tpdo 1 -i vcan0
canopenctl simulate -i vcan0 --node-id 3 --drive &             # a CiA 402 drive
canopenctl cia402 enable 3 -i vcan0 && canopenctl cia402 move 3 5000 --wait -i vcan0
canopenctl simulate -i vcan0 --lss --serial 0x1001 &           # unconfigured LSS devices
canopenctl simulate -i vcan0 --lss --serial 0x1002 &
canopenctl boot -i vcan0 --lss --slave 10 --expect 10=F7A:C0C7:10000:1001 --slave 11 --expect 11=F7A:C0C7:10000:1002
```

## Install

Binaries for every platform above are attached to each
[GitHub release](https://github.com/otfabric/canopenctl/releases). From source:

```sh
go install github.com/otfabric/canopenctl@latest
```

## Documentation

[ARCHITECTURE.md](ARCHITECTURE.md) · [TESTING.md](TESTING.md) ·
[SECURITY.md](SECURITY.md) · [LIMITATIONS.md](LIMITATIONS.md) · [ROADMAP.md](ROADMAP.md) ·
[CHANGELOG.md](CHANGELOG.md) · [CONTRIBUTING.md](CONTRIBUTING.md) ·
[RELEASE.md](RELEASE.md)

## License

MIT, see [LICENSE](LICENSE).
