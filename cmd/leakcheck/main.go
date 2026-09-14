package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/rleungx/leakcheck"
	"golang.org/x/tools/go/analysis/singlechecker"
)

// Version information, set at build time
var (
	version = "dev"     // -ldflags "-X main.version=x.y.z"
	commit  = "unknown" // -ldflags "-X main.commit=abc123"
	date    = "unknown" // -ldflags "-X main.date=2025-01-01T00:00:00Z"

	readBuildInfo = debug.ReadBuildInfo
)

func main() {
	// Keep a convenient human-readable version flag while reserving -V=full
	// for the analyzer protocol used by go vet.
	if wantsHumanVersion(os.Args[1:]) {
		fmt.Println(getVersion())
		return
	}

	// singlechecker owns flag registration and parsing. In particular, this
	// preserves -flags, -V=full, and the .cfg protocol required by go vet.
	singlechecker.Main(leakcheck.New())
}

// getVersion returns the version string
func getVersion() string {
	resolvedVersion, resolvedCommit, resolvedDate := resolveBuildMetadata()
	goVersion := strings.TrimPrefix(runtime.Version(), "go")
	result := fmt.Sprintf("leakcheck has version %s built with go%s", resolvedVersion, goVersion)
	if resolvedCommit != "unknown" {
		result += " from " + resolvedCommit
	}
	if resolvedDate != "unknown" {
		result += " on " + resolvedDate
	}
	return result
}

func wantsHumanVersion(args []string) bool {
	for _, arg := range args {
		if arg == "-V" || arg == "--version" {
			return true
		}
	}
	return false
}

func resolveBuildMetadata() (resolvedVersion, resolvedCommit, resolvedDate string) {
	resolvedVersion, resolvedCommit, resolvedDate = version, commit, date
	info, ok := readBuildInfo()
	if !ok {
		return resolvedVersion, resolvedCommit, resolvedDate
	}

	if resolvedVersion == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		resolvedVersion = strings.TrimPrefix(info.Main.Version, "v")
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			if resolvedCommit == "unknown" {
				resolvedCommit = setting.Value
				if len(resolvedCommit) > 7 {
					resolvedCommit = resolvedCommit[:7]
				}
			}
		case "vcs.time":
			if resolvedDate == "unknown" {
				resolvedDate = setting.Value
			}
		}
	}
	return resolvedVersion, resolvedCommit, resolvedDate
}
