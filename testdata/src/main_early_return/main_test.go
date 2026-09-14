package main_early_return

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	m.Run()
	return
	goleak.VerifyTestMain(m)
}

func TestUncovered(t *testing.T) {} // want "test function TestUncovered is not covered by goleak"
