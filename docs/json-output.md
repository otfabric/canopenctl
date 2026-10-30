# JSON output

`--format json` (or `CANOPENCTL_FORMAT=json`; `CANOPENCTL_OUTPUT_FORMAT` is accepted too, `_FORMAT` wins) makes every command machine
readable. Every document carries `"schemaVersion": "1.0"`; the major version
changes only with an incompatible change, and fields may be added within
1.x (ignore unknown fields). Schemas:
[schemas/result-v1.schema.json](../schemas/result-v1.schema.json) (with the
result of every command),
[schemas/record-v1.schema.json](../schemas/record-v1.schema.json),
[schemas/error-v1.schema.json](../schemas/error-v1.schema.json). The tests
validate every command's output against them.

Conventions: `data` and raw values are lower-case hexadecimal without
separators (`"10400000"`, CANopen encoding, little-endian); `key` addresses
an object as `"0x1018:02"`; COB-IDs, codes and 32-bit identity values are
`0x` strings with upper-case digits; `index` and `subIndex` are numbers.
Decoded values (`value`) are JSON numbers for integers and finite floats
(`"NaN"`, `"+Inf"`, `"-Inf"` otherwise), booleans, strings for
VISIBLE_STRING and UNICODE_STRING, and hexadecimal for OCTET_STRING, DOMAIN
and the TIME types. Timestamps are seconds with microseconds as a string,
as candump writes them (`"1700000000.001000"`).

## Results

Commands with one result write one indented document:

```json
{
  "schemaVersion": "1.0",
  "command": "sdo read",
  "result": {
    "node": 5,
    "index": 4120,
    "subIndex": 2,
    "key": "0x1018:02",
    "name": "Identity object / Product code",
    "type": "UNSIGNED32",
    "typeSource": "eds",
    "size": 4,
    "data": "10400000",
    "value": 16400,
    "block": false
  }
}
```

| `command` | `result` |
|-----------|----------|
| `sdo read`, `sdo write` | `node`, `index`, `subIndex`, `key`, `name` (with `--eds`), `type`, `typeSource` (`flag`, `eds`, `guess`, `file`), `size`, `data`, `value`, `block` (block transfer used; with `--fd` a USDO bulk transfer), `fd` (USDO, `--fd`), `verified` (`--verify`), `file` (`--output`, `--file`) |
| `sdo dump` | `node`, `eds`, `objects` (`index`, `subIndex`, `key`, `name`, `type`, `access`, `data` and `value`, or `error` and `abortCode`), `read`, `failed` |
| `nmt` | `node` (0: all), `command` (`start`, `stop`, `pre-operational`, `reset-node`, `reset-communication`), `waited`, `state` and `elapsedMs` with `--wait` |
| `scan` | `scanned`, `nodes` (`node`, `deviceType`, `profile`, `vendorId`, `productCode`, `revision`, `serialNumber` as hex strings, and `identity` with the numbers), `errors` |
| `sync` | `sent`, `periodUs`, `counterOverflow`, `cobId` |
| `time` | `sent`, `time` (RFC 3339, UTC), `days` (since 1984-01-01), `ms` (after midnight), `cobId` |
| `pdo show` | `node`, `pdos`: `kind` (`tpdo`, `rpdo`), `pdo`, `cobId`, `cobIdValue` (the raw UNSIGNED32), `valid`, `extended`, `rtrAllowed`, `active`, `transmissionType`, `transmission` (description), `inhibitTime` (100 µs), `eventTimer` (ms), `syncStart`, `bits`, `mapping` (`index`, `subIndex`, `bits`, `key`, `name`, `dummy`) |
| `pdo map` | `node` and the PDO's fields as in `pdo show`, read back after the configuration |
| `eds validate` | `file`, `dcf`, `valid`, `errors`, `warnings`, `objects`, `diagnostics` (`severity`, `section`, `key`, `message`) |
| `eds show` | `file`, `dcf`, `nodeId` (DCF), `description`, `device` (`vendorName`, `vendorNumber`, `productName`, `productNumber`, `revisionNumber`, `orderCode`, `baudRates` in kbit/s, `rxPDOs`, `txPDOs`, `lss`), `objects` (`index`, `subIndex`, `key`, `object`, `name`, `objectType`, `type`, `access`, `pdoMapping`, `default`, `lowLimit`, `highLimit`) |
| `eds convert`, `eds generate`, `dcf export` | with `--output`: `input`, `output`, `kind` (`dcf`, `eds`, `go`), `nodeId`, `bytes`, `sha256`. Without `--output` the file itself goes to stdout and there is no result document |
| `dcf apply`, `cdcf apply` | `node`, `file`, `dryRun`, `entries` (`index`, `subIndex`, `key`, `name`, `data`) |
| `cdcf convert` | with `--output`: as `eds convert`, `kind` `cdcf` |
| `cdcf show` | `file`, `format` (`cdcf`, or `dcf` for a DCF converted), `size` (bytes as a concise DCF), `entries` (as `dcf apply`) |
| `lss switch`, `select`, `inquire`, `configure`, `activate`, `identify` | `action`, and as the command sets them: `state` (`waiting`, `configuration`), `address` (an LSS address: `address` as `vendor:product:revision:serial`, `vendorId`, `productCode`, `revisionNumber`, `serialNumber`), `nodeId` (255: none), `configured`, `bitRate`, `stored`, `delayMs`, `found`, `nonConfigured` |
| `lss fastscan` | `devices` (LSS addresses) |
| `lss assign-all` | `first`, `stored`, `assignments` (LSS address fields and `nodeId`), `error` (a failure after some assignments; exit 1) |
| `mpdo configure` | `node`, `kind`, `pdo`, `mode` (`SAM`, `DAM`), `cobId`, `inhibitTime` |
| `mpdo scanner`, `mpdo dispatcher` | `node`, `list`, `index` (of the list object), `entries` (text), `scanner` (`index`, `subIndex`, `blockSize`) or `dispatcher` (`producer`, `producerIndex`, `producerSubIndex`, `index`, `subIndex`, `blockSize`) |
| `boot` | `master`, `lss`, `started`, `slaves` (`node`, `mandatory`, `config`, `entries`, `expected`, `booted`, `status`, `code` (the CiA 302 boot status letter A..O), `error`, `lss` (the address the LSS step gave the node-ID to)), `booted`, `failed`, `error`, `elapsedMs` |
| `store`, `restore` | `node`, `action`, `group`, `groupName`, `key` (the sub-object written), `reset` (`restore --reset`) |
| `program download`, `program control`, `program status` | `node`, `program`, `action` (`download`, `status`, or the control command), `file`, `bytes`, `identification`, `state`, `stateValue`, `flashStatus`, `flashText`, `elapsedMs` |
| `cia402 state` and the other `cia402` commands | `node`, `action` (not for state), `state` (CiA 402 name), `statusword`, `status` (names of the set statusword bits), `mode` and `modeName`, `position`, `velocity` (when the drive has the objects), `faultCode` and `faultText` in Fault |
| `version` | `version`, `tag`, `commit`, `buildDate`, `goCanopen`, `goCanbus`, `go`, `platform` |

## Streams

Streaming commands write JSON Lines: one compact object per line with
`schemaVersion` and `type`, and a `summary` record at the end.

| Command | `type` | Fields |
|---------|--------|--------|
| `monitor`, `decode` | `message` | the frame (`timestamp`, `interface`, `id`, `idHex`, `extended`, `remote`, `len` (up to 64), `fd` and `brs` (CAN FD), `data`), `kind` (`nmt`, `sync`, `time`, `emcy`, `tpdo`, `rpdo`, `sdo-request`, `sdo-response`, `usdo-request`, `usdo-response`, `heartbeat`, `boot-up`, `guard-request`, `guard-response`, `lss-master`, `lss-slave`, `unknown`), `node` (for a USDO message its sender), `summary` (one line), `error` (malformed), and one object for the service: `nmt`, `sync`, `timeStamp`, `emcy` (`code`, `codeText`, `register`, `registerNames`, `msef`, `reset`, and `fd` for a CANopen FD emergency: `logicalDevice`, `ciaSpec`, `status`, `ms`, `days`, `time`), `pdo`, `sdo` (command, phase, object, data, abort code and text, block transfer fields; segments carry the object of their transfer), `usdo` (`client`, `dest`, `destination`, `cs`, `command`, `session`, `index`, `subIndex`, `hasIndex`, `dataType`, `size`, `counter`, `tProc`, `abortCode`, `abortText`, `data`), `errorControl`, `lss` |
| | `summary` | `frames`, `shown`, `malformed`, `dropped`, `kinds` (count per kind) |
| `heartbeat watch` | `heartbeat` | `timestamp`, `node`, `event` (`boot-up`, `state-change`, `timeout`, `resumed`), `state`, `previous` |
| | `summary` | `events`, `timeouts`, `bootUps`, `nodes` (`node`, `state`, `bootUps`, `lost`, `lastSeen`, `timeoutMs`) |
| `emcy watch` | `emcy` | `timestamp`, `node`, `cobId`, `code`, `codeText`, `register`, `registerNames`, `msef`, `reset`, `fd` (`--fd`: the CANopen FD fields as in `monitor`) |
| | `summary` | `received`, `shown`, `resets`, `invalid` |
| `pdo watch` | `pdo` | `timestamp`, `node`, `kind`, `pdo`, `cobId`, `data`, `values` (`index`, `subIndex`, `key`, `name`, `type`, `raw`, `value`) |
| | `summary` | `received`, `errors` |
| `mpdo watch` | `mpdo` | `timestamp`, `cobId`, `mode` (`sam`, `dam`, `malformed`), `node` (DAM: the consumer, 0 for all; SAM: the producer), `index`, `subIndex`, `key`, `name` (`--eds`), `data` (the 4 data bytes; the frame for a malformed one), `error` |
| | `summary` | `received`, `malformed`, `dropped` |
| `simulate` | `ready` | `node` (255: an unconfigured LSS node), `state`, `objects`, `source` (the EDS, or `built-in`), `drive`, `lss`, `fd`, `store`, `program` |
| | `state` | `timestamp`, `node`, `from`, `to`, `command` (the NMT command that caused it) |
| | `summary` | `node`, `state`, `ticks`, `stats` (go-canopen's node.Stats: NMT, SDO server, heartbeat, EMCY, SYNC, TIME, PDO counters) |

```json
{"schemaVersion":"1.0","type":"message","timestamp":"1700000000.002000","interface":"vcan0","id":1541,"idHex":"605","extended":false,"remote":false,"len":8,"data":"4008100000000000","kind":"sdo-request","node":5,"summary":"SDO request node 5: initiate upload 0x1008:00","sdo":{"client":true,"cs":2,"command":"initiate upload","phase":"initiate","index":4104,"subIndex":0,"hasIndex":true}}
```

## Errors

A fatal error is one document on stdout, and the exit code tells its kind
([exitcodes.md](exitcodes.md)). An SDO failure carries the abort; a CiA 402
failure the drive's state:

```json
{
  "schemaVersion": "1.0",
  "error": {
    "kind": "abort",
    "code": "sdo_abort",
    "message": "canopen: SDO abort 0x06020000: object does not exist in the object dictionary (remote, object 0x5FFF:00)",
    "abort": {
      "code": "0x06020000",
      "text": "object does not exist in the object dictionary",
      "index": 24575,
      "subIndex": 0,
      "remote": true
    }
  }
}
```

`abort` of a USDO (CANopen FD) failure: `code` and `text` are the SDO
equivalent of the one-byte USDO abort code, which `usdoCode` holds
(`"0x12"`). `drive`: `operation`, `state`, `statusword`, `faultCode`. Results written
before a failure (a stream's records) stay on stdout before the error
document.
