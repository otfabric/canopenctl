// SPDX-License-Identifier: MIT

package buildinfo

import (
	"runtime"
	"runtime/debug"
	"testing"
)

const sha = "1802d0f9afe8642d1e278120b76fcdfac967cb9e"

var goPlat = runtime.Version() + ", " + runtime.GOOS + "/" + runtime.GOARCH

func withBuildInfo(t *testing.T, bi *debug.BuildInfo) {
	t.Helper()
	old := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) { return bi, bi != nil }
	t.Cleanup(func() { readBuildInfo = old })
}

func TestTextAllFields(t *testing.T) {
	withBuildInfo(t, nil)
	got := Resolve("canopenctl", "0.6.0", "v0.6.0", sha, "2026-11-05T15:41:49Z").Text()
	want := "canopenctl 0.6.0 (tag v0.6.0, commit " + sha + ", built 2026-11-05T15:41:49Z, " + goPlat + ")"
	if got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	// A leading v of the injected version is dropped.
	if v := Resolve("canopenctl", "v0.6.0", "v0.6.0", sha, "x").Version; v != "0.6.0" {
		t.Fatalf("version %q", v)
	}
}

func TestTextWithoutTag(t *testing.T) {
	withBuildInfo(t, nil)
	got := Resolve("canopenctl", "0.6.0", "", sha, "2026-11-05T15:41:49Z").Text()
	want := "canopenctl 0.6.0 (commit " + sha + ", built 2026-11-05T15:41:49Z, " + goPlat + ")"
	if got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func TestDevFallback(t *testing.T) {
	withBuildInfo(t, nil)
	got := Resolve("canopenctl", "", "", "", "").Text()
	want := "canopenctl dev (commit unknown, built unknown, " + goPlat + ")"
	if got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	if v := Resolve("canopenctl", "dev", "", "", "").Version; v != "dev" {
		t.Fatalf("version %q", v)
	}
}

func TestBuildInfoFallback(t *testing.T) {
	withBuildInfo(t, &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}, Settings: []debug.BuildSetting{
		{Key: "vcs.revision", Value: sha}, {Key: "vcs.modified", Value: "true"},
	}})
	got := Resolve("canopenctl", "", "", "", "").Text()
	want := "canopenctl dev (commit " + sha + "-dirty, built unknown, " + goPlat + ")"
	if got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}

	// go install ...@v0.6.0: the module version, no VCS stamp.
	withBuildInfo(t, &debug.BuildInfo{Main: debug.Module{Version: "v0.6.0"}})
	i := Resolve("canopenctl", "", "", "", "")
	if i.Version != "0.6.0" || i.Commit != "unknown" {
		t.Fatalf("%+v", i)
	}

	// go build in a checkout: a pseudo-version, not a release.
	for _, v := range []string{"v0.5.1-0.20261107212138-ed2b5bc8f960+dirty", "v0.5.1-0.20261107212138-ed2b5bc8f960", "v0.0.0-20261107212138-ed2b5bc8f960", "v0.6.0+dirty"} {
		withBuildInfo(t, &debug.BuildInfo{Main: debug.Module{Version: v}})
		if got := Resolve("canopenctl", "", "", "", "").Version; got != "dev" {
			t.Fatalf("%s: version %q", v, got)
		}
	}

	// A clean checkout: no -dirty.
	withBuildInfo(t, &debug.BuildInfo{Settings: []debug.BuildSetting{
		{Key: "vcs.revision", Value: sha}, {Key: "vcs.modified", Value: "false"},
	}})
	if c := Resolve("canopenctl", "", "", "", "").Commit; c != sha {
		t.Fatalf("commit %q", c)
	}

	// Injected values win over the build information.
	if i := Resolve("canopenctl", "1.0.0", "", "abc", ""); i.Version != "1.0.0" || i.Commit != "abc" {
		t.Fatalf("%+v", i)
	}
}

func TestJSON(t *testing.T) {
	withBuildInfo(t, nil)
	got := Resolve("canopenctl", "0.6.0", "v0.6.0", sha, "2026-11-05T15:41:49Z").JSON()
	want := `{"name":"canopenctl","version":"0.6.0","tag":"v0.6.0","commit":"` + sha +
		`","built":"2026-11-05T15:41:49Z","libraries":[],"go":"` + runtime.Version() + `","platform":"` + runtime.GOOS + "/" + runtime.GOARCH + `"}`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	got = Resolve("canopenctl", "", "", "", "").JSON()
	want = `{"name":"canopenctl","version":"dev","tag":"","commit":"unknown","built":"unknown","libraries":[],"go":"` +
		runtime.Version() + `","platform":"` + runtime.GOOS + "/" + runtime.GOARCH + `"}`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestLibrariesFormatting(t *testing.T) {
	base := Info{Name: "canopenctl", Version: "0.6.0", Tag: "v0.6.0", Commit: sha, Built: "2026-11-05T15:41:49Z", Go: "go1.27.3", Platform: "darwin/arm64"}
	head := "canopenctl 0.6.0 (tag v0.6.0, commit " + sha + ", built 2026-11-05T15:41:49Z, "
	jhead := `{"name":"canopenctl","version":"0.6.0","tag":"v0.6.0","commit":"` + sha + `","built":"2026-11-05T15:41:49Z","libraries":`
	jtail := `,"go":"go1.27.3","platform":"darwin/arm64"}`
	for _, c := range []struct {
		libs       []Library
		text, json string
	}{
		{nil, "", `[]`},
		{[]Library{}, "", `[]`},
		{[]Library{{"go-canbus", "0.8.0"}}, "go-canbus 0.8.0, ", `[{"name":"go-canbus","version":"0.8.0"}]`},
		{[]Library{{"go-canbus", "0.8.0"}, {"go-canopen", "0.5.1"}}, "go-canbus 0.8.0, go-canopen 0.5.1, ",
			`[{"name":"go-canbus","version":"0.8.0"},{"name":"go-canopen","version":"0.5.1"}]`},
	} {
		i := base
		i.Libraries = c.libs
		if got, want := i.Text(), head+c.text+"go1.27.3, darwin/arm64)"; got != want {
			t.Errorf("text\n got  %q\n want %q", got, want)
		}
		if got, want := i.JSON(), jhead+c.json+jtail; got != want {
			t.Errorf("json\n got  %s\n want %s", got, want)
		}
	}
}

func TestLibrariesFromBuildInfo(t *testing.T) {
	withBuildInfo(t, &debug.BuildInfo{
		Main: debug.Module{Path: "github.com/otfabric/go-self", Version: "(devel)"},
		Deps: []*debug.Module{
			{Path: "github.com/spf13/cobra", Version: "v1.10.1"},
			{Path: "github.com/otfabric/go-canopen", Version: "v0.5.1"},
			{Path: "github.com/otfabric/go-self", Version: "v1.0.0"},
			{Path: "github.com/otfabric/go-canbus", Version: "v0.8.0", Replace: &debug.Module{Path: "../go-canbus"}},
			{Path: "github.com/otfabric/go-alpha", Version: "v0.1.0", Replace: &debug.Module{Path: "x", Version: "v0.2.0-0.20261107212138-ed2b5bc8f960"}},
			{Path: "github.com/otfabric/canbusctl", Version: "v0.9.0"},
		},
	})
	got := Resolve("canopenctl", "0.6.0", "", sha, "").Libraries
	want := []Library{{"go-alpha", "0.2.0-0.20261107212138-ed2b5bc8f960"}, {"go-canbus", "0.8.0"}, {"go-canopen", "0.5.1"}}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k := range want {
		if got[k] != want[k] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
