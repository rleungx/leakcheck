# Makefile for leakcheck

# Version information  
VERSION := $(shell if git describe --tags >/dev/null 2>&1; then git describe --tags | sed 's/^v//'; else echo "dev"; fi)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# Build flags
LDFLAGS = -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

all: build

build:
	go build -ldflags "$(LDFLAGS)" -o bin/leakcheck cmd/leakcheck/main.go

test-deps:
	cd testdata/src && go mod vendor && cd ../..

test: test-deps
	go test ./...

test-coverage: test-deps
	go test ./... -coverprofile=coverage.out

self-check: build
	go vet -vettool=$(CURDIR)/bin/leakcheck ./...
	./bin/leakcheck ./...

bench:
	go test -run '^$$' -bench '^BenchmarkAnalyzer$$' -benchmem .

lint:
	golangci-lint run ./...

tidy:
	go mod tidy

.PHONY: all build tidy lint test-deps test test-coverage self-check bench
