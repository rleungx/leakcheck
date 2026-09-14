package main_goroutine

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	go goleak.VerifyTestMain(m)
}

func TestUncovered(t *testing.T) {} // want "test function TestUncovered is not covered by goleak"
