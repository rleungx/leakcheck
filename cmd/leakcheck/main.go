package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/rleungx/leakcheck"
	"golang.org/x/tools/go/analysis/singlechecker"
)

// Version information, set at build time
var (
	version = "0.1.0"   // -ldflags "-X main.version=x.y.z"
	commit  = "unknown" // -ldflags "-X main.commit=abc123"
	date    = "unknown" // -ldflags "-X main.date=2025-01-01T00:00:00Z"
)

func main() {
	// Define flags
	var (
		excludePackages = flag.String("exclude-packages", "", "comma-separated list of package patterns to exclude (supports regex)")
		excludeFiles    = flag.String("exclude-files", "", "comma-separated list of file patterns to exclude (supports regex)")
		showHelp        = flag.Bool("h", false, "show help message")
		showVersion     = flag.Bool("V", false, "show version information")
	)

	// Custom usage function
	flag.Usage = func() {
		showHelpMessage()
	}

	// Parse flags
	flag.Parse()

	// Handle help flag
	if *showHelp {
		showHelpMessage()
		return
	}

	// Handle version flag
	if *showVersion {
		fmt.Println(getVersion())
		return
	}

	// If no arguments provided after flags, show help
	if flag.NArg() == 0 {
		showHelpMessage()
		return
	}

	// Create analyzer with configuration
	config := &leakcheck.Config{
		ExcludePackages: *excludePackages,
		ExcludeFiles:    *excludeFiles,
	}
	configuredAnalyzer := leakcheck.NewWithConfig(config)

	// Prepare os.Args for singlechecker (remove our custom flags)
	// Keep only the program name and the remaining arguments
	newArgs := []string{os.Args[0]}
	newArgs = append(newArgs, flag.Args()...)
	os.Args = newArgs

	// Run the analyzer using singlechecker
	singlechecker.Main(configuredAnalyzer)
}

// getVersion returns the version string
func getVersion() string {
	// Format: "leakcheck has version x.y.z built with goX.Y.Z from abc123 on 2025-01-01T00:00:00Z"
	goVersion := strings.TrimPrefix(runtime.Version(), "go")

	if version == "dev" && commit != "unknown" {
		return fmt.Sprintf("leakcheck has version %s built with go%s from %s on %s",
			version, goVersion, commit, date)
	}

	if version != "dev" {
		return fmt.Sprintf("leakcheck has version %s built with go%s from %s on %s",
			version, goVersion, commit, date)
	}

	return fmt.Sprintf("leakcheck has version %s built with go%s", version, goVersion)
}

func showHelpMessage() {
	fmt.Println(`leakcheck - Goroutine Leak Detection Linter

A static analysis tool that ensures all Go test functions are properly covered by goleak
for goroutine leak detection.

USAGE:
    leakcheck [flags] [packages]

FLAGS:
    -exclude-packages string
            Comma-separated list of package patterns to exclude (supports regex)
    -exclude-files string  
            Comma-separated list of file patterns to exclude (supports regex)
    -h  Show this help message
    -V  Show version information

EXAMPLES:
    leakcheck ./...                                    # Analyze all packages
    leakcheck ./pkg/server ./pkg/client               # Analyze specific packages
    leakcheck -exclude-packages=".*_test" ./...       # Exclude test packages
    leakcheck -exclude-files="*_mock_test.go" ./...   # Exclude mock test files

For more information, visit: https://github.com/rleungx/leakcheck`)
}
