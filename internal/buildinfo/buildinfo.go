// SPDX-License-Identifier: MIT

// Package buildinfo resolves the build metadata of canopenctl and renders
// the output of `canopenctl version`: one text line, or one JSON line.
//
// The values come from the link-time variables of package main (-X
// main.version, main.tag, main.commit, main.buildDate); what was not
// injected falls back to the Go build information (go install ...@vX.Y.Z,
// the VCS stamp of go build) and then to "dev" or "unknown".
package buildinfo

import (
	"encoding/json"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
)

// Info is the resolved build metadata. Every field is set; Tag is ""
// when no tag was injected.
type Info struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Tag      string `json:"tag"`
	Commit   string `json:"commit"`
	Built    string `json:"built"`
	Go       string `json:"go"`
	Platform string `json:"platform"`
}

// readBuildInfo is debug.ReadBuildInfo, replaced in tests.
var readBuildInfo = debug.ReadBuildInfo

// Resolve returns the metadata of the binary name from the injected
// values (empty when not injected).
func Resolve(name, version, tag, commit, buildDate string) Info {
	i := Info{
		Name:     name,
		Version:  strings.TrimPrefix(strings.TrimSpace(version), "v"),
		Tag:      strings.TrimSpace(tag),
		Commit:   strings.TrimSpace(commit),
		Built:    strings.TrimSpace(buildDate),
		Go:       runtime.Version(),
		Platform: runtime.GOOS + "/" + runtime.GOARCH,
	}
	if i.Version == "dev" {
		i.Version = ""
	}
	if i.Version == "" || i.Commit == "" {
		if bi, ok := readBuildInfo(); ok && bi != nil {
			if i.Version == "" && released(bi.Main.Version) {
				i.Version = strings.TrimPrefix(bi.Main.Version, "v")
			}
			if i.Commit == "" {
				i.Commit = vcsCommit(bi)
			}
		}
	}
	if i.Version == "" {
		i.Version = "dev"
	}
	if i.Commit == "" {
		i.Commit = "unknown"
	}
	if i.Built == "" {
		i.Built = "unknown"
	}
	return i
}

// pseudo matches the pseudo-version (and +dirty) that go build stamps from
// the VCS: v0.5.1-0.20261107212138-ed2b5bc8f960+dirty.
var pseudo = regexp.MustCompile(`\d{14}-[0-9a-f]{12}(\+dirty)?$`)

// released reports whether v, the module version of the build
// information, is a release (go install ...@vX.Y.Z), not "(devel)" or a
// pseudo-version of a local build.
func released(v string) bool {
	return v != "" && v != "(devel)" && !strings.HasSuffix(v, "+dirty") && !pseudo.MatchString(v)
}

// vcsCommit returns vcs.revision of the build information, with -dirty
// when vcs.modified is true.
func vcsCommit(bi *debug.BuildInfo) string {
	var rev string
	var dirty bool
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev != "" && dirty {
		rev += "-dirty"
	}
	return rev
}

// Text returns the version line (without a newline):
//
//	canopenctl 0.6.0 (tag v0.6.0, commit <sha>, built <date>, go1.26.0, linux/amd64)
func (i Info) Text() string {
	var b strings.Builder
	b.WriteString(i.Name + " " + i.Version + " (")
	if i.Tag != "" {
		b.WriteString("tag " + i.Tag + ", ")
	}
	b.WriteString("commit " + i.Commit + ", built " + i.Built + ", " + i.Go + ", " + i.Platform + ")")
	return b.String()
}

// JSON returns the version as one compact JSON object (without a
// newline), keys in the order name, version, tag, commit, built, go,
// platform.
func (i Info) JSON() string {
	b, _ := json.Marshal(i) // a struct of strings always marshals
	return string(b)
}
