# AGENTS.md — canopenctl

## What this is

Command-line toolkit for CANopen networks built on [go-canopen](https://github.com/otfabric/go-canopen) and [go-canbus](https://github.com/otfabric/go-canbus): monitor/decode, SDO/USDO, NMT, LSS, CiA 302 boot, heartbeat/EMCY, PDO/MPDO, EDS/DCF, program download, CiA 402 drives, device simulation. Pure Go, one static binary. Profile: **cli**.

## Organisation standards

This repository follows **otfabric/engineering policy 1.0**, profile **cli** (`.otfabric/policy.yaml`, mode `baseline`). Where this file and the organisation standards disagree, this file wins here — say so in your report.

## Where the truth lives

| Question | Source |
|---|---|
| Exit codes (script-facing API) | docs/exitcodes.md |
| JSON output contract | docs/json-output.md, schemas/*.schema.json |
| Command reference | docs/cli.md (generated: `make docs`) |
| CANopen behaviour | go-canopen (its LIMITATIONS.md and INTEROPERABILITY.md) |
| Release history | RELEASE.md (`## vX.Y.Z` … `---`) |

## Hard rules

1. Exit codes and JSON output are a public contract: changing a code or a field is a breaking change with a RELEASE.md note.
2. State-changing transmissions on a non-virtual interface require `--yes` (exit 9 `safety` otherwise). Read-only is the default.
3. Never lower `COVERAGE_MIN`, add lint suppressions, or skip/delete tests to make CI pass.
4. `docs/cli.md` is regenerated, never hand-edited.
5. go-canopen and go-canbus are pinned to released tags; no `replace` directives.

## Commands

| When you change… | Run |
|---|---|
| anything | `make check` |
| a command's flags or help | `make docs` |
| SocketCAN paths (Linux) | `make test-vcan` |
| input parsing | `make fuzz` |
| interop-relevant behaviour | `make interop` (Docker, vcan0) |

## Definition of Done

`make check` green; e2e coverage for new commands; docs/cli.md regenerated; exit codes and JSON documented; RELEASE.md section for user-visible changes.

## Git and releases

Branches and PRs. Do not push to `main`, tag or run the Release workflow unless the maintainer asks in the session.
