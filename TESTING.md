# Testing

| Command | What |
|---------|------|
| `make test` | All tests with the race detector; Linux vcan tests skip (loudly) without vcan0 |
| `make test-vcan` | The same on Linux, one package at a time, failing (not skipping) when vcan0 is missing (`CANOPENCTL_REQUIRE_VCAN=1`) |
| `make coverage` | Whole-module coverage (`-coverpkg=./...`), gated at 90 % |
| `make e2e` | End-to-end tests of the built binary (Linux parts need vcan0) |
| `make interop` | The binary against the canopen-interop images (Docker, vcan0) |
| `make fuzz` | Fuzz targets of the parsed inputs: values per data type, PDO mappings, node lists, indices/sub-indices/types/COB-IDs, LSS addresses and MPDO list entries |
| `make cross` | vet for every release platform, linux/arm (32-bit) included |
| `make check` | fmt, staticcheck, golangci-lint, vet, cross, govulncheck, test, coverage |

**In-process command tests** (`cmd/*_test.go`) run the real command tree
through `cmd.Run` with captured streams and check exit codes, text and JSON.
Every command runs against go-canopen nodes on the in-process virtual bus: a
CiA 401 I/O module from `testdata/io401.eds`, simulate's built-in device and
a CiA 402 drive (`node.WithDrive`); for v0.2.0 also LSS slaves
(`node.WithLSS`, unconfigured and configured, with a memory store), MPDO
producers and consumers with scanner and dispatcher lists, devices with
store/restore and program download (`node.WithStore`, `node.WithProgram`),
the CiA 302 boot-up (with the LSS step) and CANopen FD nodes
(`node.WithFD` on CAN FD ports of the virtual bus) operated over USDO. Streams (monitor, watches) run while the
test produces traffic or events until the command's `--count` is reached. A
command that runs into the test's 30 s limit fails the test (a stream that
waited for nothing cannot pass silently).

**Golden files** (`cmd/testdata/golden`, regenerate with
`go test ./cmd -update`) pin `decode` (text and JSON Lines), `eds show`,
`eds validate`, `pdo show`, `sdo read`, `scan`, `dcf apply --dry-run` and the
error documents of an SDO abort and of the safety refusal.

**JSON Schemas** (`schemas/`): `TestSchemasGolden` validates every golden
JSON file and `TestSchemasCommands` runs every command with `--format json`
(results, streams, error documents) and validates its output.

**vcan tests** (`cmd/vcan_linux_test.go`) drive a node on vcan0 with the
commands on their own sockets: on vcan no `--yes` is needed. Setup:

```sh
sudo modprobe vcan
sudo ip link add vcan0 type vcan && sudo ip link set vcan0 up
```

**End-to-end tests** (`e2e/`) build the binary and drive it as separate
processes: EDS → DCF → EDS → Go code chains, decoding a piped trace, error
documents and exit codes, simulate on its own virtual bus; on Linux a
simulated CiA 401 node process on vcan0 (EDS, heartbeat, ticking input, a
delayed EMCY) operated by scan, sdo read/write/dump, nmt with `--wait`, pdo
show/map/watch, sync and time (with a monitor process), dcf export → apply,
reset, while heartbeat watch, emcy watch and monitor processes listen; a
simulated CiA 402 drive process operated by cia402; two unconfigured
`simulate --lss` processes found by `lss fastscan` and booted by `boot
--lss`; a `simulate --fd` process operated over USDO while `monitor --fd`
listens (needs the CAN FD MTU 72 on vcan0, which `scripts/ci-can-modules.sh`
sets); and the safety refusal.
With `CANOPENCTL_BIN` set they use a prebuilt binary. To run them in a Linux
VM without Go (for example Lima, with the repository mounted):

```sh
GOOS=linux GOARCH=arm64 go test -c -o bin/e2e.test ./e2e
GOOS=linux GOARCH=arm64 go test -c -o bin/cmd.test ./cmd
GOOS=linux GOARCH=arm64 go build -o bin/canopenctl-linux-arm64 .
limactl shell canvm -- bash -lc 'cd ~/…/canopenctl/cmd && CANOPENCTL_REQUIRE_VCAN=1 ../bin/cmd.test'
limactl shell canvm -- bash -lc 'cd ~/…/canopenctl/e2e && CANOPENCTL_REQUIRE_VCAN=1 CANOPENCTL_BIN=$PWD/../bin/canopenctl-linux-arm64 ../bin/e2e.test -test.v'
```

**Interop tests** (`interop/`, build tag `interop`) run the binary against
the canopen-interop images (CANopenNode, Lely CANopen, python canopen; the
release `InteropRelease`, or `CANOPENCTL_INTEROP_TAG=dev` for locally built
images, or `CANOPENCTL_INTEROP_<ADAPTER>_IMAGE`) on vcan0:

- canopenctl against each stack's **device** (the shared
  `interop-device.eds`, node 5): SDO read/write expedited, segmented and
  block, DOMAIN round trip, aborts (write-only, read-only, no object, value
  range), dump, scan, pdo show, nmt start `--wait`, pdo watch of the ticking
  TPDO1, pdo map of TPDO2 and its values, emcy watch, heartbeat watch, dcf
  apply of `interop-device-node5.dcf`, checked against the device's own dump.
- each stack's **client** against `canopenctl simulate` with the same EDS:
  SDO upload/download (expedited, segmented, block), abort, NMT start,
  pdo-receive, pdo-configure (read back with pdo show), SYNC (counted by
  monitor), listen for boot-up/heartbeat and EMCY, and python canopen's
  CiA 402 client enabling `simulate --drive`.
- the stacks' **EDS checkers** against `eds validate` on the fixtures.

A capability an image lacks is a recorded skip, never a pass; deliberate
differences are recorded as known differences (`CANOPENCTL_INTEROP_REPORT`
writes the matrix). `CANOPENCTL_REQUIRE_INTEROP=1` turns a missing image or
interface into a failure.
