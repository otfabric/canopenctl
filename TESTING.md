# Testing

| Command | What |
|---------|------|
| `make test` | All tests with the race detector; Linux vcan tests skip (loudly) without vcan0 |
| `make test-vcan` | The same on Linux, one package at a time, failing (not skipping) when vcan0 is missing (`CANOPENCTL_REQUIRE_VCAN=1`) |
| `make coverage` | Whole-module coverage (`-coverpkg=./...`), gated at 90 % |
| `make coverage-pkg-check` | Every package at least `COVER_PKG_MIN` (90 %); lowest measured in CI: 92.4 % (cmd). The vcan job runs it |
| `make e2e` | End-to-end tests of the built binary (Linux parts need vcan0) |
| `make interop` | The binary against the canopen-interop images (Docker, vcan0) |
| `make fuzz` | Fuzz targets of the parsed inputs: values per data type, PDO mappings, node lists, indices/sub-indices/types/COB-IDs, LSS addresses and MPDO list entries |
| `make cross` | vet for every release platform, linux/arm (32-bit) included |
| `make check` | fmt, staticcheck, golangci-lint, vet, cross, govulncheck, test, coverage, coverage-pkg-check |

Per-package override: the root package main is not gated (`COVER_PKG_SKIP`). It only calls `cmd.Execute`; the e2e suite runs the built binary, which `go test` coverage does not instrument.

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

Contract 1.1 (canopen-interop v0.2.0) adds, for the v0.2.0 commands:

- **LSS** (`lss_test.go`): `lss` against each stack's LSS slave (`device
  --node-id 255 --serial N`; CANopenNode and Lely): identify (non-configured
  and an address range), select (and an absent address: exit 5), inquire,
  bit timing (the device's `bit-rate` event), configure node-ID (active
  node-ID checked by the device's `node-id` event and by SDO 0x1018:04 of
  the new node), inquire node-ID, store (the device ends with the stored
  node-ID), Fastscan of an unconfigured and of a configured device, and
  assign-all of two unconfigured devices with `--store`. Each stack's LSS
  master (`lss --step`, all three) against `canopenctl simulate --lss`:
  inquire, configure node-ID, store (the restarted `simulate --store`
  comes up with the stored node-ID), Fastscan, Fastscan with only a
  configured node (exit 3) and assign-all (CANopenNode).
- **MPDO** (`mpdo_test.go`, Lely): `mpdo configure` of a Lely device's RPDO
  1 as DAM consumer and as SAM consumer with `mpdo dispatcher`, checked by
  Lely's `mpdo-send` (to the node, to another node, to all; dispatched and
  not dispatched producers) and SDO reads; `mpdo configure` of TPDO 1 as
  SAM producer with `mpdo scanner` (read back by SDO); `mpdo watch`
  decoding Lely's DAM- and SAM-MPDOs.
- **CiA 302 boot-up** (`boot_test.go`): `boot` of each stack's device (a
  concise DCF written independently of go-canopen, `--expect`; the values
  read back, the node operational by heartbeat), with the identity of
  `interop-device-other-product.eds` (error status M, exit 1, not
  started), with `--lss` for the stacks' unconfigured LSS slaves, and of
  one network of a device of every stack plus a `simulate` node. Lely's
  `boot` master booting `canopenctl simulate` (concise DCF of dcfgen,
  started) and refusing the other product (error status M).
- **Concise DCFs and the remaining CiA 302 commands** (`cdcf_test.go`):
  `cdcf convert` byte for byte against Lely's `cdcf --generator dcfgen`
  (seven configuration values, the DCF mounted at `/work`) and `--generator
  objects` (0x2000..0x2004 of a DCF), `cdcf show` of Lely's file; `cdcf
  apply` of a converted DCF to each stack's device, read back by SDO and
  in the device's dump; `store`, `restore` and `program` on each device
  where it has 0x1010/0x1011 and 0x1F50/0x1F51 (probed by SDO); `--fd`.

A capability an image lacks is a recorded skip, never a pass — also a
device object the contract's device lacks (probed: abort 0x06020000) and
CANopen FD, which no image declares. Deliberate differences are recorded as
known differences with the observation (`interop/known_test.go`,
`edsDifferences`); any other deviation fails. `CANOPENCTL_INTEROP_REPORT`
writes the matrix. `CANOPENCTL_REQUIRE_INTEROP=1` turns a missing image or
interface into a failure.

To run the suite in a Linux VM without Go (Lima, the repository mounted
read-only, Docker usable through the docker group):

```sh
GOOS=linux GOARCH=arm64 go build -o bin/canopenctl-linux-arm64 .
GOOS=linux GOARCH=arm64 go test -c -tags interop -o bin/interop.test ./interop
limactl shell canvm -- bash -lc 'cd /tmp && sg docker -c "CANOPENCTL_REQUIRE_VCAN=1 CANOPENCTL_REQUIRE_INTEROP=1 \
  CANOPENCTL_BIN=$HOME/…/canopenctl/bin/canopenctl-linux-arm64 CANOPENCTL_INTEROP_REPORT=/tmp/interop-report.json \
  $HOME/…/canopenctl/bin/interop.test -test.v"'
```

### Interop results

canopen-interop v0.2.0 (contract 1.1, images pinned by digest), Linux 7.0
arm64 (Lima), vcan0, 2026-10-30; the full suite run twice with identical counts, the new scenarios three more times:

| Adapter | Pass | Known difference | Skip | Fail | Before (v0.2.0 suite: pass / known / skip) |
|---------|-----:|-----:|-----:|-----:|---|
| canopennode | 48 | 2 | 23 | 0 | 27 / 0 / 10 |
| lely | 62 | 3 | 8 | 0 | 33 / 1 / 3 |
| pythoncanopen | 41 | 0 | 32 | 0 | 33 / 0 / 4 |
| canopennode+lely+pythoncanopen (boot-network) | 1 | 0 | 0 | 0 | – |
| **Total** | **152** | **5** | **63** | **0** | 93 / 1 / 17 |

Skips are capabilities the stack (or the contract's device) lacks:
CANopenNode has no PDO client, SYNC producer, EDS checker, CiA 402 client,
MPDO, CiA 302 boot master or concise DCF generator; Lely no `pdo-configure`,
`sync`, CiA 402 client or assign-all; python canopen's device no block
transfer, TPDO, limits or LSS slave, and no MPDO, boot master or concise DCF
generator; no device of the contract has 0x1010/0x1011 or 0x1F50/0x1F51,
and no image declares CANopen FD.

Known differences (each observed on the bus with candump, and recorded by
canopen-interop or go-canopen's interop suite too):

| Adapter | Scenario | Difference |
|---------|----------|------------|
| canopennode | lss-identify-non-configured | CANopenNode v4.1's LSS slave does not answer "identify non-configured remote slave" (0x4C sent three times, no 0x50 on 0x7E4) while it answers Fastscan; optional for slaves in CiA 305 |
| canopennode | lss-identify-remote | Likewise "identify remote slave" (0x46..0x4B sent three times, no 0x4F) |
| lely | lss-activate-on-waiting | Lely's LSS slave applies a configured node-ID only at the next NMT reset communication: no boot-up 0x728 after `lss configure --address`, a boot-up after `nmt reset-comm all` |
| lely | lss-fastscan-configured | Lely's LSS slave answers Fastscan whatever its node-ID: `lss fastscan` lists the configured node 5 (0x4F answers on 0x7E4) |
| lely | eds-object-links | Lely's dcf-tools reject `[xxxxObjectLinks]` sections (v0.1 suite) |
