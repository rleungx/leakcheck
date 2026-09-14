package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
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
	flag.Var(versionFlag{}, "V", "print version and exit (-V=full for go vet)")
	flag.Var(versionFlag{}, "version", "print version and exit")

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

// Let flag.Parse distinguish version flags from option values and positional
// arguments. singlechecker preserves an existing -V flag registration.
type versionFlag struct{}

func (versionFlag) String() string   { return "false" }
func (versionFlag) IsBoolFlag() bool { return true }

func (versionFlag) Set(value string) error {
	if value == "full" {
		if err := printToolVersion(); err != nil {
			return err
		}
	} else {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		if !enabled {
			return nil
		}
		fmt.Println(getVersion())
	}
	os.Exit(0)
	return nil
}

// go vet uses this protocol to key its analysis cache by executable content.
func printToolVersion() error {
	path, err := os.Executable()
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }() // Closing a read-only executable cannot lose data.
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return err
	}
	_, err = fmt.Printf("%s version devel buildID=%x\n", path, hash.Sum(nil))
	return err
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
