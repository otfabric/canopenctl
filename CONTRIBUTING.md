# Contributing

1. Work on a branch; open a pull request against `main`.
2. Run `make check` (and `make test-vcan` and `make e2e` on Linux with
   vcan0) before pushing.
3. New commands follow the existing shape: a `newXxxCmd(a *app)` constructor,
   validation before any transmission, typed errors from `internal/errs`, a
   result type with `Text()` (and `Headers()`/`Rows()` for tables), tests
   through `cmd.Run` against go-canopen nodes on the virtual bus, the JSON
   output in `schemas/` and validated by `TestSchemasCommands`.
4. Anything that changes a device or the network goes through
   `cli.TransmitCheck` (`busFlags.check`) and is bounded (`--count`,
   `--duration`, `--timeout`).
5. CANopen logic (protocols, the object dictionary, EDS, PDO packing)
   belongs in go-canopen, CAN logic in go-canbus, not here.
6. Regenerate the reference after changing help texts: `make docs`.
7. Add an SPDX header (`// SPDX-License-Identifier: MIT`) to new Go files.
8. Commit messages: imperative subject, a body that says why.
