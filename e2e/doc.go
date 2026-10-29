// SPDX-License-Identifier: MIT

// Package e2e drives the built canopenctl binary end to end as separate
// processes: EDS/DCF chains, trace decoding and error documents
// everywhere; on Linux with vcan0, a simulated node process (from an EDS,
// and a CiA 402 drive) operated by every command in its own process.
package e2e
