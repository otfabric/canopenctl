// SPDX-License-Identifier: MIT

// Command canopenctl monitors, configures, tests and simulates CANopen
// networks: SDO, NMT, heartbeat, EMCY, SYNC, TIME, PDOs, EDS/DCF files and
// CiA 402 drives.
package main

import (
	"os"

	"github.com/otfabric/canopenctl/cmd"
)

// Injected at link time: -X main.version=... etc.
var (
	version   = "dev"
	tag       = ""
	commit    = ""
	buildDate = ""
)

func main() {
	cmd.SetBuildMeta(version, tag, commit, buildDate)
	os.Exit(cmd.Execute())
}
