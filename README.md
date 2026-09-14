# leakcheck - Goroutine Leak Detection Linter

![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/rleungx/leakcheck/go.yml)
![Codecov](https://img.shields.io/codecov/c/github/rleungx/leakcheck)
![GitHub License](https://img.shields.io/github/license/rleungx/leakcheck)

A static analysis tool that ensures all Go test functions are properly covered by [goleak](https://github.com/uber-go/goleak) for goroutine leak detection.

## Features

- Detects missing `goleak` imports and `defer goleak.VerifyNone(t)` calls in test functions
- Validates `TestMain(m *testing.M)` with `goleak.VerifyTestMain(m)` setup  
- Supports package aliases and configurable exclusion patterns
- Concurrent analysis with configurable performance settings
- Regex and glob pattern matching for flexible exclusions

## Quick Start

```bash
# Install
go install github.com/rleungx/leakcheck/cmd/leakcheck@latest

# Usage
leakcheck ./...                                          # Analyze all packages
leakcheck -exclude-files="*mock*" ./...                  # Exclude files matching pattern
leakcheck -exclude-packages="vendor,internal" ./...      # Exclude multiple packages
leakcheck -concurrency=8 -timeout=10m ./...              # Custom performance settings
```

## Examples

### Missing goleak Import
```go
// ❌ Will be flagged
func TestSomething(t *testing.T) {
    // goleak not imported
}
```

### Missing defer Statement
```go
import "go.uber.org/goleak"

// ❌ Missing defer
func TestSomething(t *testing.T) {
    // test logic
}

// ✅ Correct
func TestCorrect(t *testing.T) {
    defer goleak.VerifyNone(t)
    // test logic
}
```

### TestMain Coverage
```go
// ❌ TestMain without goleak
func TestMain(m *testing.M) {
    os.Exit(m.Run())
}

// ✅ Correct - covers all tests
func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m)
}
```

`TestMain(t *testing.T)` is an ordinary test, not a package entry point, and
requires its own `defer goleak.VerifyNone(t)` like any other test.

### Coverage Paths

Checks must cover every reachable exit path within the function. A check only
inside a conditional branch, after an early return, or in unreachable code does
not cover the function. Both branches may establish coverage separately:

```go
func TestBranches(t *testing.T) {
    if useCustomOptions {
        defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
    } else {
        defer goleak.VerifyNone(t)
    }
    // test logic
}
```

The analysis follows branches, loops, and explicit exits, including standard
panic/Goexit/testing termination calls. `os.Exit` and `log.Fatal` bypass deferred
checks. It does not follow helpers, closures, or goroutines to establish coverage,
and assumes unknown calls return normally. Dynamic panics and correlations
between nonconstant conditions are not modeled, so this is a conservative check
of explicit control flow, not a proof that every runtime execution checks for
leaks or is leak-free.

### Exclusion Examples

```bash
# Exclude specific files
leakcheck -exclude-files="mock_test.go,generated_test.go" ./...

# Exclude files with regex
leakcheck -exclude-files=".*mock.*,.*_gen\.go$" ./...

# Exclude multiple packages
leakcheck -exclude-packages="vendor,internal,testdata" ./...

# Exclude packages with regex
leakcheck -exclude-packages=".*test.*,vendor" ./...
```

Patterns are comma-separated. Plain words use substring matching. Patterns with
a `*` not immediately preceded by `.` and no other regex operators
(`+?^${}()[]|\`) are whole-string globs: `mock_*` matches
`mock_helpers_test.go`, not `real_mock_test.go`; dots in `*_mock_test.go` and
`*.*` are literal. Patterns whose wildcards are all `.*`, or which contain other
regex operators, use regular expressions, such as `legacy_.*` or
`^mock_.*\.go$`. A glob that does not match is never retried as a regex. File
patterns are checked against both the full path and the basename; package
patterns use the import path.

## Development

```bash
git clone https://github.com/rleungx/leakcheck.git
cd leakcheck

# Build
make build

# Run tests
make test

# Run tests with coverage
make test-coverage

# Verify the analyzer against this repository and the go vet protocol
make self-check

# Run linter
make lint

# Tidy dependencies
make tidy
```

## License

Apache License 2.0 - see [LICENSE](LICENSE) file for details.
