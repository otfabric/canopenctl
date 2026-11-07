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
		`","built":"2026-11-05T15:41:49Z","go":"` + runtime.Version() + `","platform":"` + runtime.GOOS + "/" + runtime.GOARCH + `"}`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	got = Resolve("canopenctl", "", "", "", "").JSON()
	want = `{"name":"canopenctl","version":"dev","tag":"","commit":"unknown","built":"unknown","go":"` +
		runtime.Version() + `","platform":"` + runtime.GOOS + "/" + runtime.GOARCH + `"}`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}
