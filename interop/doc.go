// SPDX-License-Identifier: MIT

// Package interop tests the canopenctl binary against the independent
// CANopen stacks of the canopen-interop images (CANopenNode, Lely
// CANopen, python canopen): canopenctl as SDO client, NMT master,
// heartbeat, EMCY and PDO consumer and DCF tool against each stack's
// device; each stack's SDO client, NMT master, consumers, SYNC producer
// and CiA 402 client against canopenctl simulate; and the EDS checkers
// against eds validate. Contract 1.1 scenarios: canopenctl's LSS master
// against the stacks' LSS slaves and the stacks' LSS masters against
// simulate --lss; mpdo configure/scanner/dispatcher/watch with Lely's MPDO
// devices; boot (CiA 302) of the stacks' devices, of a mixed network and
// with the LSS step, and Lely's boot master booting simulate; cdcf convert
// byte for byte against Lely's generators and cdcf apply to every device;
// store/restore, program and --fd where a device has them. The tests
// carry the interop build tag and need Docker and a vcan interface:
//
//	go test -tags interop -p 1 ./interop
//
// A capability an image lacks is a skip, recorded as such in the report
// (CANOPENCTL_INTEROP_REPORT), never a pass.
package interop
