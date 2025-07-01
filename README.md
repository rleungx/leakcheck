# leakcheck - Goroutine Leak Detection Linter

A static analysis tool that ensures all Go test functions are properly covered by [goleak](https://github.com/uber-go/goleak) for goroutine leak detection.

## Features

- Detects missing `goleak` imports and `defer goleak.VerifyNone(t)` calls in test functions
- Validates `TestMain(m *testing.M)` with `goleak.VerifyTestMain(m)` setup  
- Supports package aliases and configurable exclusion patterns

## Quick Start

```bash
# Install
go install github.com/rleungx/leakcheck/cmd/leakcheck@latest

# Usage
leakcheck ./...                                    # Analyze all packages
leakcheck -exclude-files="*mock*" ./...           # Exclude files
leakcheck -exclude-packages="vendor" ./...        # Exclude packages
leakcheck -h                                       # Show help
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

# Run linter
make lint

# Tidy dependencies
make tidy
```

## License

Apache License 2.0 - see [LICENSE](LICENSE) file for details.
