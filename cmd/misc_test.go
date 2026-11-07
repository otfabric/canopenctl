// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/otfabric/canopenctl/internal/buildinfo"
)

func TestVersionAndCompletion(t *testing.T) {
	fresh(t)
	plat := runtime.Version() + ", " + runtime.GOOS + "/" + runtime.GOARCH
	r := run(t, "", "version")
	want(t, r, 0, "canopenctl ", ", built unknown, go-canbus ", ", go-canopen ", ", "+plat+")")
	if strings.Count(r.out, "\n") != 1 || strings.Contains(r.out, "tag ") {
		t.Fatalf("version: %q", r.out)
	}
	SetBuildMeta("0.6.0", "v0.6.0", "1802d0f9afe8642d1e278120b76fcdfac967cb9e", "2026-11-05T15:41:49Z")
	defer SetBuildMeta("", "", "", "")
	bi := buildinfo.Resolve("canopenctl", "0.6.0", "v0.6.0", "1802d0f9afe8642d1e278120b76fcdfac967cb9e", "2026-11-05T15:41:49Z")
	line, js := bi.Text()+"\n", bi.JSON()+"\n"
	for _, w := range []string{"canopenctl 0.6.0 (tag v0.6.0, commit 1802d0f9afe8642d1e278120b76fcdfac967cb9e, built 2026-11-05T15:41:49Z, go-canbus ", ", go-canopen ", ", " + plat + ")"} {
		if !strings.Contains(line, w) {
			t.Fatalf("version line %q lacks %q", line, w)
		}
	}
	for _, w := range []string{`{"name":"canopenctl","version":"0.6.0","tag":"v0.6.0","commit":"1802d0f9afe8642d1e278120b76fcdfac967cb9e","built":"2026-11-05T15:41:49Z","libraries":[{"name":"go-canbus","version":"`, `{"name":"go-canopen","version":"`, `"go":"` + runtime.Version() + `"`} {
		if !strings.Contains(js, w) {
			t.Fatalf("version JSON %q lacks %q", js, w)
		}
	}
	for _, c := range []struct {
		args []string
		out  string
	}{
		{[]string{"version"}, line},
		{[]string{"version", "--format", "text"}, line},
		{[]string{"version", "--format", "table"}, line},
		{[]string{"version", "--json"}, js},
		{[]string{"version", "--format", "json"}, js},
		{[]string{"version", "--json", "--format", "text"}, js},
	} {
		if r := run(t, "", c.args...); r.code != 0 || r.out != c.out {
			t.Fatalf("%v: exit %d\n got %q\nwant %q", c.args, r.code, r.out, c.out)
		}
	}
	t.Setenv("CANOPENCTL_FORMAT", "json")
	if r := run(t, "", "version"); r.out != js {
		t.Fatalf("CANOPENCTL_FORMAT=json: %q", r.out)
	}
	t.Setenv("CANOPENCTL_FORMAT", "")
	for _, sh := range []string{"bash", "zsh", "fish", "powershell"} {
		want(t, run(t, "", "completion", sh), 0, "canopenctl")
	}
	want(t, run(t, "", "completion", "tcsh"), 3, "unknown shell")
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
	want(t, run(t, "", "version"), 0, `"name":"canopenctl"`)
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
	if r := run(t, "", "version"); !strings.Contains(r.out, `"name":"canopenctl"`) {
		t.Fatalf("OUTPUT_FORMAT ignored: %q", r.out)
	}
	t.Setenv("CANOPENCTL_FORMAT", "text")
	if r := run(t, "", "version"); strings.Contains(r.out, `"name":"canopenctl"`) {
		t.Fatalf("FORMAT does not win: %q", r.out)
	}
}
