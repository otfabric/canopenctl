// SPDX-License-Identifier: MIT

//go:build interop

package interop

import (
	"fmt"
	"testing"
)

// knownDifferences are the documented differences of the contract 1.1
// scenarios (adapter/scenario): a scenario that observes one ends as a
// known difference with the reason and what it observed, never as a pass;
// any other deviation fails. Each was observed on the bus (candump) or in
// the stack's output before it was listed.
var knownDifferences = map[string]string{
	"canopennode/lss-identify-non-configured": "CANopenNode v4.1's LSS slave does not implement \"identify non-configured remote slave\" (0x4C, optional in CiA 305 for slaves): no answer 0x50 on 0x7E4 (candump), while it answers Fastscan; go-canopen's interop suite records the same difference",
	"canopennode/lss-identify-remote":         "CANopenNode v4.1's LSS slave does not implement \"identify remote slave\" (0x46..0x4B, optional in CiA 305 for slaves): no answer 0x4F on 0x7E4 (candump), while it answers Fastscan; go-canopen's interop suite records the same difference",
	"lely/lss-activate-on-waiting":            "Lely's LSS slave (co_lss.c) applies a configured node-ID only at the next NMT reset communication (pending until then): after \"lss configure --address\" switched it back to waiting it reports no node-id event; canopenctl's nmt reset-comm activates it (lss-configure-node-id passes); canopen-interop known difference",
	"lely/lss-fastscan-configured":            "Lely's LSS slave answers LSS Fastscan whatever its node-ID (co_lss_fastscan checks the identity only): lss fastscan lists the configured node; CANopenNode's and go-canopen's slaves take part only while their node-ID is 255; canopen-interop known difference",
}

// differ ends a scenario that observed a deviation: as a known difference
// when adapter/name is listed (with what was observed), else as a
// failure.
func differ(t *testing.T, adapter, name, format string, args ...any) {
	t.Helper()
	msg := fmt.Sprintf(format, args...)
	if why, ok := knownDifferences[adapter+"/"+name]; ok {
		knownDifference(t, why+" [observed: "+msg+"]")
		return
	}
	t.Fatal(msg)
}
