// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"os"
	"runtime/debug"
	"strings"
	"testing"
	"time"
)

func TestVersionAndCompletion(t *testing.T) {
	fresh(t)
	want(t, run(t, "", "version"), 0, "canopenctl dev", "go-canopen", "go-canbus")
	SetBuildMeta("v9.9.9", "v9.9.9", "abc1234", "2026-10-29T00:00:00Z")
	defer SetBuildMeta("", "", "", "")
	want(t, run(t, "", "version", "--format", "json"), 0, `"version": "v9.9.9"`, `"commit": "abc1234"`, `"goCanopen"`)
	want(t, run(t, "", "version"), 0, "canopenctl v9.9.9 (abc1234)")
	for _, sh := range []string{"bash", "zsh", "fish", "powershell"} {
		want(t, run(t, "", "completion", sh), 0, "canopenctl")
	}
	want(t, run(t, "", "completion", "tcsh"), 3, "unknown shell")
	info := &debug.BuildInfo{Deps: []*debug.Module{
		{Path: "a", Version: "v1.0.0"},
		{Path: "b", Version: "v0.0.0", Replace: &debug.Module{Path: "../b"}},
		{Path: "c", Version: "v0.0.0", Replace: &debug.Module{Path: "x", Version: "v1.2.3"}},
	}}
	if depVersion(info, "a") != "v1.0.0" || depVersion(info, "b") != "v0.0.0 (replaced by ../b)" || depVersion(info, "c") != "v1.2.3" || depVersion(info, "d") != "(unknown)" {
		t.Fatal("depVersion")
	}
}

func TestUsageAndFormat(t *testing.T) {
	fresh(t)
	want(t, run(t, "", "bogus"), 2, "unknown command", "--help")
	want(t, run(t, "", "bogus", "--format", "json"), 2, `"kind": "usage"`)
	want(t, run(t, "", "sdo", "read", "--format=json"), 2, `"kind": "usage"`)
	want(t, run(t, "", "version", "--format", "yaml"), 3, "invalid --format")
	want(t, run(t, "", "version", "--slcan-bitrate", "12345"), 3, "--slcan-bitrate")
	want(t, run(t, "", "--help"), 0, "Read-only by default")
	t.Setenv("CANOPENCTL_FORMAT", "json")
	want(t, run(t, "", "version"), 0, `"schemaVersion": "1.0"`)
	if formatOf([]string{"--format", "nonsense"}) != "text" {
		t.Fatal("formatOf")
	}
}

func TestInterfaces(t *testing.T) {
	fresh(t)
	t.Setenv("CANOPENCTL_INTERFACE", "virtual")
	want(t, run(t, "", "nmt", "start", "all"), 0, "NMT start")
	t.Setenv("CANOPENCTL_INTERFACE", "")
	// An slcan adapter that cannot be opened: transport failure.
	want(t, run(t, "", "sdo", "read", "5", "1000", "0", "-i", "slcan:/nonexistent/tty"), 4, "")
	want(t, run(t, "", "sdo", "read", "5", "1000", "0", "-i", "slcan:tcp://127.0.0.1:1", "--slcan-bitrate", "0"), 4, "")
	want(t, run(t, "", "nmt", "start", "all", "-i", "slcan:/nonexistent/tty", "--yes"), 4, "")
	if !linux() {
		// No SocketCAN off Linux: unsupported.
		want(t, run(t, "", "monitor", "-i", "can0", "--duration", "10ms"), 8, "")
		want(t, run(t, "", "nmt", "start", "all", "-i", "can0"), 8, "")
	}
	want(t, run(t, "", "nmt", "start", "all", "-i", "nosuchcan0"), map[bool]int{true: 4, false: 8}[linux()], "")
}

func TestExecute(t *testing.T) {
	old := os.Args
	defer func() { os.Args = old }()
	os.Args = []string{"canopenctl", "version"}
	if Execute() != 0 {
		t.Fatal("Execute")
	}
}

func TestBounded(t *testing.T) {
	ctx, cancel := bounded(context.Background(), time.Millisecond)
	defer cancel()
	<-ctx.Done()
	if !ended(ctx, ctx.Err()) {
		t.Fatal("ended")
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()
	if !ended(ctx2, ctx2.Err()) || ended(context.Background(), context.Canceled) {
		t.Fatal("ended canceled")
	}
}

// CANOPENCTL_OUTPUT_FORMAT is accepted; CANOPENCTL_FORMAT is canonical and wins.
func TestOutputFormatEnv(t *testing.T) {
	t.Setenv("CANOPENCTL_FORMAT", "")
	t.Setenv("CANOPENCTL_OUTPUT_FORMAT", "json")
	if r := run(t, "", "version"); !strings.Contains(r.out, `"schemaVersion"`) {
		t.Fatalf("OUTPUT_FORMAT ignored: %q", r.out)
	}
	t.Setenv("CANOPENCTL_FORMAT", "text")
	if r := run(t, "", "version"); strings.Contains(r.out, `"schemaVersion"`) {
		t.Fatalf("FORMAT does not win: %q", r.out)
	}
}
