// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"
	"time"

	"github.com/otfabric/go-canopen/cia402"
	"github.com/otfabric/go-canopen/node"
)

func driveNode(t *testing.T) *node.Node {
	t.Helper()
	return builtinNode(t, 3, true, node.WithDrive(cia402.DriveConfig{}, time.Millisecond))
}

func TestCiA402(t *testing.T) {
	fresh(t)
	n := driveNode(t)
	fast := []string{"-i", "virtual", "--poll", "1ms"}
	args := func(a ...string) []string { return append(a, fast...) }
	eventually(t, "switch on disabled", func() bool { return n.Drive().State() == cia402.SwitchOnDisabled })
	want(t, run(t, "", args("cia402", "state", "3")...), 0, "node 3: Switch on disabled")
	want(t, run(t, "", args("cia402", "enable", "3")...), 0, "enable done; node 3: Operation enabled", "operation enabled")
	want(t, run(t, "", args("cia402", "move", "3", "1000", "--wait", "--velocity", "100000", "--accel", "1000000", "--decel", "1000000")...), 0, "move done", "position 1000")
	want(t, run(t, "", "cia402", "move", "-i", "virtual", "--poll", "1ms", "--relative", "--immediate", "--wait", "--velocity", "100000", "--accel", "1000000", "--decel", "1000000", "--move-timeout", "3s", "--", "3", "-500"), 0, "position 500")
	want(t, run(t, "", args("cia402", "velocity", "3", "2000", "--format", "json")...), 0, `"action": "velocity"`, `"modeName": "PV (Profile velocity)"`)
	want(t, run(t, "", args("cia402", "mode", "3", "csp")...), 0, "mode CSP")
	want(t, run(t, "", args("cia402", "mode", "3", "1")...), 0, "mode PP")
	want(t, run(t, "", args("cia402", "quick-stop", "3")...), 0, "quick-stop done")
	want(t, run(t, "", args("cia402", "disable", "3")...), 0, "Switch on disabled")
	// A fault: enable fails with exit 12 and the drive state; fault-reset.
	n.Drive().InjectFault(0x2310)
	eventually(t, "fault", func() bool { return n.Drive().State() == cia402.Fault })
	r := run(t, "", args("cia402", "state", "3", "--format", "json")...)
	want(t, r, 0, `"state": "Fault"`, `"faultCode": "0x2310"`)
	want(t, run(t, "", args("cia402", "enable", "3")...), 12, "fault")
	r = run(t, "", args("cia402", "enable", "3", "--format", "json")...)
	want(t, r, 12, `"code": "drive_fault"`, `"drive": {`, `"faultCode": "0x2310"`)
	want(t, run(t, "", args("cia402", "fault-reset", "3")...), 0, "Switch on disabled")
	n.Drive().InjectFault(0x2310)
	eventually(t, "fault", func() bool { return n.Drive().State() == cia402.Fault })
	want(t, run(t, "", args("cia402", "enable", "3", "--reset-fault")...), 0, "Operation enabled")
	// Motion needs Operation enabled.
	want(t, run(t, "", args("cia402", "disable", "3")...), 0)
	want(t, run(t, "", args("cia402", "move", "3", "10")...), 12, "")
	// Invalid input.
	want(t, run(t, "", args("cia402", "mode", "3", "warp")...), 3, "invalid mode")
	want(t, run(t, "", args("cia402", "move", "3", "far")...), 3, "invalid target")
	want(t, run(t, "", args("cia402", "velocity", "3", "fast")...), 3, "invalid velocity")
	want(t, run(t, "", args("cia402", "state", "300")...), 3, "node-ID")
	want(t, run(t, "", args("cia402", "state", "3", "--step-timeout", "0s")...), 3, "--step-timeout")
	want(t, run(t, "", "cia402", "enable", "3", "-i", "slcan:/dev/null"), 9, "--yes")
	// No drive at the node: the statusword object is missing.
	want(t, run(t, "", args("cia402", "state", "9", "--timeout", "20ms")...), 5, "")
}

func TestCiA402NoDrive(t *testing.T) {
	fresh(t)
	ioNode(t, 5)
	want(t, run(t, "", "cia402", "state", "5", "-i", "virtual"), 11, "0x06020000")
	r := run(t, "", "cia402", "enable", "5", "-i", "virtual", "--format", "json")
	want(t, r, 11, `"abort": {`)
	if m, err := parseMode("-3"); err != nil || m != -3 {
		t.Fatal(m, err)
	}
	if len(statusBits(0)) != 0 || len(statusBits(0x0237)) != 6 {
		t.Fatal(statusBits(0))
	}
}
