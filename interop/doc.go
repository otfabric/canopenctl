// SPDX-License-Identifier: MIT

// Package interop tests the canopenctl binary against the independent
// CANopen stacks of the canopen-interop images (CANopenNode, Lely
// CANopen, python canopen): canopenctl as SDO client, NMT master,
// heartbeat, EMCY and PDO consumer and DCF tool against each stack's
// device; each stack's SDO client, NMT master, consumers, SYNC producer
// and CiA 402 client against canopenctl simulate; and the EDS checkers
// against eds validate. The tests carry the interop build tag and need
// Docker and a vcan interface:
//
//	go test -tags interop -p 1 ./interop
//
// A capability an image lacks is a skip, recorded as such in the report
// (CANOPENCTL_INTEROP_REPORT), never a pass.
package interop
