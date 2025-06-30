# leakcheck - Goroutine Leak Detection Linter

A **golangci-lint compatible** static analysis tool that ensures all Go test functions are properly covered by [goleak](https://github.com/uber-go/goleak) for goroutine leak detection.

## Why Use This Linter?

Goroutine leaks are a common source of memory leaks and flaky tests in Go applications. The `goleak` library helps detect these leaks, but developers often forget to add the necessary `defer goleak.VerifyNone(t)` calls or `goleak.VerifyTestMain(m)` setup. This linter automatically catches missing coverage across your entire codebase.

## What It Detects

### 1. Missing goleak Import
```go
// ❌ Will be flagged
func TestSomething(t *testing.T) {
    // test logic without goleak import
}
```

### 2. Missing defer goleak.VerifyNone(t)
```go
import "go.uber.org/goleak"

// ❌ Will be flagged - has import but missing defer
func TestSomething(t *testing.T) {
    // test logic without defer
}

// ✅ Correct usage
func TestSomethingElse(t *testing.T) {
    defer goleak.VerifyNone(t)
    // test logic
}
```

### 3. Incomplete TestMain Setup
```go
// ❌ Will be flagged - TestMain exists but doesn't call VerifyTestMain
func TestMain(m *testing.M) {
    os.Exit(m.Run())
}

// ✅ Correct usage - covers all tests in the package
func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m)
}
```

### 4. Mixed Coverage Patterns
The linter intelligently handles:
- Multiple test files in the same package
- Packages where some tests have `defer goleak.VerifyNone(t)` and others don't
- Cross-file TestMain coverage (TestMain in one file covers tests in other files)

## Quick Start

### Install
```bash
go install github.com/rleungx/leakcheck/cmd/leakcheck@latest
```

### Run on Your Project
```bash
# Analyze current directory and subdirectories
leakcheck ./...

# Analyze specific packages
leakcheck ./pkg/server ./pkg/client

# Show help
leakcheck -h
```

### Example Output
```
pkg/server/handler_test.go:15:1: test function TestCreateUser is not covered by goleak (missing defer goleak.VerifyNone(t))
pkg/client/client_test.go:8:1: test function TestConnect is not covered by goleak (goleak not imported)
```

## Integration with golangci-lint

This linter is built using the `go/analysis` framework and can be integrated into golangci-lint.

### Integration Steps

1. **Add to golangci-lint** (when integrated):
   ```yaml
   # .golangci.yml
   linters:
     enable:
       - leakcheck
   ```

2. **Manual Integration** (for custom builds):
   ```go
   // pkg/golinters/leakcheck/leakcheck.go
   package leakcheck

   import (
       "github.com/rleungx/leakcheck"
       "golang.org/x/tools/go/analysis"
   )

   func New() *analysis.Analyzer {
       return leakcheck.Analyzer
   }
   ```

## Development and Testing

### Build from Source
```bash
git clone https://github.com/rleungx/leakcheck.git
cd leakcheck
go build -o leakcheck ./cmd/leakcheck
```

### Run Tests
```bash
# Run all tests
go test -v

# Run specific test
go test -v -run TestBasic
```

### Test Scenarios

The project includes comprehensive test cases covering various scenarios:

| Scenario | Description | Test Package |
|----------|-------------|--------------|
| **Basic Mixed Coverage** | Some tests have `defer goleak.VerifyNone(t)`, others don't | `basic` |
| **No Import** | Test files without goleak import | `no_import` |
| **TestMain Complete** | TestMain with `goleak.VerifyTestMain(m)` covers all tests | `main_with_verify` |
| **TestMain Incomplete** | TestMain exists but doesn't call `goleak.VerifyTestMain(m)` | `main_without_verify` |
| **Multiple Files** | Multiple test files in same package with mixed coverage | `multiple_files` |
| **Multiple Files + TestMain** | Multiple test files with TestMain in one file | `multiple_files_with_main` |

### Manual Testing
```bash
# Build the tool first
go build -o leakcheck ./cmd/leakcheck

# Test different scenarios from testdata/src directory
cd testdata/src
../../leakcheck ./basic               # Mixed coverage scenario
../../leakcheck ./no_import           # No goleak import scenario
../../leakcheck ./main_with_verify    # Complete TestMain coverage
../../leakcheck ./main_without_verify # Incomplete TestMain scenario
../../leakcheck ./multiple_files      # Multi-file package scenario
../../leakcheck ./multiple_files_with_main # Multi-file with TestMain
```

## How It Works

The linter uses Go's AST (Abstract Syntax Tree) analysis to:

1. **Identify test files** by checking for `_test.go` suffix
2. **Parse imports** to detect goleak package usage
3. **Analyze test functions** (functions starting with "Test") for proper coverage
4. **Handle TestMain** patterns and their scope across packages
5. **Report violations** with precise file locations and helpful messages

## Error Messages

| Message | Meaning | Solution |
|---------|---------|----------|
| `goleak not imported` | Test file doesn't import goleak | Add `import "go.uber.org/goleak"` |
| `missing defer goleak.VerifyNone(t)` | Test function lacks defer statement | Add `defer goleak.VerifyNone(t)` at function start |
| `TestMain exists but doesn't call goleak.VerifyTestMain` | Incomplete TestMain setup | Replace `m.Run()` with `goleak.VerifyTestMain(m)` |

## Best Practices

### Recommended Pattern #1: Per-Test Coverage
```go
func TestSomething(t *testing.T) {
    defer goleak.VerifyNone(t)
    // your test logic here
}
```

### Recommended Pattern #2: Package-Wide Coverage
```go
func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m)
}

// All other test functions in the package are automatically covered
func TestSomething(t *testing.T) {
    // no defer needed - covered by TestMain
}
```

### With Options
```go
func TestWithOptions(t *testing.T) {
    defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("some.background.function"))
    // test logic
}
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass (`go test -v`)
6. Commit your changes (`git commit -am 'Add amazing feature'`)
7. Push to the branch (`git push origin feature/amazing-feature`)
8. Open a Pull Request

## License

[MIT License](LICENSE)

## Related Projects

- [goleak](https://github.com/uber-go/goleak) - The goroutine leak detector this linter ensures coverage for
- [golangci-lint](https://golangci-lint.run/) - Fast linters runner for Go
- [go/analysis](https://pkg.go.dev/golang.org/x/tools/go/analysis) - The framework this linter is built on
