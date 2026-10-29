# Process exit codes

canopenctl's exit codes are stable for scripting. A fatal error is rendered once:
as an `error:` line on stderr (an SDO abort adds an `abort` line with the
code's meaning), or, with `--format json`, as a single JSON error document on
stdout (see [json-output.md](json-output.md)). `--format` is honoured also for
usage errors.

| Code | Kind | Meaning |
|------|------|---------|
| 0 | | Success |
| 1 | | Findings: a result was written and it reports problems — `eds validate` found errors (or warnings with `--strict`), `decode` met malformed CANopen messages, `sdo dump` or `dcf export` could not read some objects, `scan` found a node that fails on 0x1018, `heartbeat watch` saw a heartbeat timeout, `sdo write --verify` read back another value. No error document follows |
| 2 | `usage` | Unknown command or flag, missing required flag or argument, no `--interface` |
| 3 | `invalid_input` | A value, node-ID, index, data type, mapping, file or combination of flags that does not make sense. Nothing was transmitted |
| 4 | `transport` | The interface does not exist, is down, or failed while in use |
| 5 | `timeout` | A device did not answer: an SDO response (`sdo_timeout`, with the abort canopenctl sent), `nmt --wait`, a CiA 402 step (`--step-timeout`) |
| 6 | `protocol` | The device violated the protocol (`sdo_protocol`: wrong toggle bit, unexpected command; the abort canopenctl sent is in `abort`), or sent data that does not fit the requested data type (`data_type_mismatch`) |
| 7 | `permission` | A privilege is missing to open the interface |
| 8 | `unsupported` | Not available here: SocketCAN on macOS/Windows |
| 9 | `safety` | Refused for safety: a state-changing transmission on an interface that is not virtual without `--yes` |
| 10 | `output`, `internal` | The result could not be written, the command was interrupted before a result, or something unexpected happened |
| 11 | `abort` | **The device refused an SDO transfer**: it sent an SDO abort. The JSON error's `abort` object has the code (`"0x06020000"`), CiA 301's text, the object (`index`, `subIndex`) and `"remote": true` |
| 12 | `device` | The device is in a state that prevents the operation: a CiA 402 drive in Fault (`drive_fault`) or not in the required state (`drive_state`; the error's `drive` object has the state, statusword and fault code), an inactive PDO (`pdo_inactive`) |

`kind` selects the exit code; `code` in the JSON error is a stable, finer
identifier:

| `code` | Kinds |
|--------|-------|
| `invalid_input` | `usage`, `invalid_input` |
| `invalid_value` | `invalid_input`: a value that does not parse as its data type |
| `invalid_eds` | `invalid_input`: an EDS/DCF that cannot be read or built into a dictionary |
| `invalid_capture` | `invalid_input`: a trace that cannot be read |
| `file_open_failed` | `invalid_input` |
| `interface_unavailable` | `transport` |
| `io_failed` | `transport` |
| `timeout`, `sdo_timeout`, `no_response` | `timeout` |
| `sdo_protocol`, `data_type_mismatch` | `protocol` |
| `permission_denied` | `permission` |
| `unsupported` | `unsupported` |
| `confirmation_required` | `safety` |
| `sdo_abort` | `abort` |
| `drive_fault`, `drive_state`, `pdo_inactive`, `identity_mismatch`, `not_producer` | `device` |
| `output_failed` | `output` |
| `canceled`, `internal_unexpected` | `internal` |

## SDO abort codes

The abort code a device sends tells why it refused (CiA 301 7.2.4.3.17); the
most common:

| Code | Meaning |
|------|---------|
| 0x05030000 | toggle bit not alternated |
| 0x05040000 | SDO protocol timed out |
| 0x06010000 | unsupported access to an object |
| 0x06010001 | attempt to read a write-only object |
| 0x06010002 | attempt to write a read-only object |
| 0x06020000 | object does not exist in the object dictionary |
| 0x06040041 | object cannot be mapped to the PDO |
| 0x06040042 | the number and length of the objects to be mapped would exceed the PDO length |
| 0x06040043 | general parameter incompatibility |
| 0x06070010 | data type does not match, length of service parameter does not match |
| 0x06090011 | sub-index does not exist |
| 0x06090030 | invalid value for parameter |
| 0x06090031 / 0x06090032 | value written too high / too low |
| 0x08000020 | data cannot be transferred or stored to the application |
| 0x08000021 | ... because of local control |
| 0x08000022 | ... because of the present device state |

**Streams.** `monitor`, `heartbeat watch`, `emcy watch`, `pdo watch` and
`simulate` end normally (exit 0, or 1 for `heartbeat watch` after a timeout)
at `--count`, `--duration` or Ctrl-C (SIGINT/SIGTERM).

**Transmitting commands** validate everything before the first frame: an
invocation that exits 2, 3, 8 or 9 has sent nothing.
