package main

import (
	"runtime/debug"
	"strings"
	"testing"

	"go.uber.org/goleak"
)

func TestResolveBuildMetadataFromModuleVersion(t *testing.T) {
	defer goleak.VerifyNone(t)

	oldVersion, oldCommit, oldDate := version, commit, date
	oldReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		version, commit, date = oldVersion, oldCommit, oldDate
		readBuildInfo = oldReadBuildInfo
	})

	version, commit, date = "dev", "unknown", "unknown"
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main: debug.Module{Version: "v1.2.3"},
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "1234567890abcdef"},
				{Key: "vcs.time", Value: "2026-09-14T00:00:00Z"},
			},
		}, true
	}

	got := getVersion()
	for _, want := range []string{"version 1.2.3", "from 1234567", "on 2026-09-14T00:00:00Z"} {
		if !strings.Contains(got, want) {
			t.Fatalf("getVersion() = %q, want it to contain %q", got, want)
		}
	}
}

func TestInjectedBuildMetadataTakesPrecedence(t *testing.T) {
	defer goleak.VerifyNone(t)

	oldVersion, oldCommit, oldDate := version, commit, date
	oldReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		version, commit, date = oldVersion, oldCommit, oldDate
		readBuildInfo = oldReadBuildInfo
	})

	version, commit, date = "2.0.0", "abcdef0", "2026-09-14T01:00:00Z"
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}, true
	}

	got := getVersion()
	for _, want := range []string{"version 2.0.0", "from abcdef0", "on 2026-09-14T01:00:00Z"} {
		if !strings.Contains(got, want) {
			t.Fatalf("getVersion() = %q, want it to contain %q", got, want)
		}
	}
}

func TestWantsHumanVersion(t *testing.T) {
	defer goleak.VerifyNone(t)

	for _, args := range [][]string{{"-V"}, {"--version"}, {"./...", "-V"}} {
		if !wantsHumanVersion(args) {
			t.Fatalf("wantsHumanVersion(%q) = false, want true", args)
		}
	}
	for _, args := range [][]string{nil, {"-V=full"}, {"--help"}} {
		if wantsHumanVersion(args) {
			t.Fatalf("wantsHumanVersion(%q) = true, want false", args)
		}
	}
}
