package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/rleungx/leakcheck"
	"golang.org/x/tools/go/analysis/unitchecker"
)

func main() {
	// Check for help flag first
	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "--help" {
			showHelp()
			return
		}
		if arg == "-V" || arg == "--version" {
			fmt.Println(getVersion())
			return
		}
	}

	// Custom flags for leakcheck (avoid conflicts with unitchecker flags)
	var (
		excludePackages = flag.String("exclude-packages", "", "comma-separated list of package patterns to exclude (supports regex)")
		excludeFiles    = flag.String("exclude-files", "", "comma-separated list of file patterns to exclude (supports regex)")
	)

	// Parse flags without conflicting with unitchecker
	flag.Parse()

	// Create analyzer with configuration
	config := &leakcheck.Config{
		ExcludePackages: *excludePackages,
		ExcludeFiles:    *excludeFiles,
	}
	configuredAnalyzer := leakcheck.NewWithConfig(config)

	// Run the analyzer using unitchecker
	unitchecker.Main(configuredAnalyzer)
}

// getVersion returns the version string based on git information
func getVersion() string {
	// Try git describe first (tags or commit)
	if version := getGitVersion("git", "describe", "--tags", "--exact-match", "HEAD"); version != "" {
		return "leakcheck " + version
	}

	// Fallback to commit hash
	if version := getGitVersion("git", "rev-parse", "--short", "HEAD"); version != "" {
		return "leakcheck " + version
	}

	// Final fallback
	return "leakcheck v0.0.0-dev"
}

// getGitVersion executes git command and returns trimmed output
func getGitVersion(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func showHelp() {
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
