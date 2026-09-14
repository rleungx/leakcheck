package main_closure

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	_ = func() { goleak.VerifyTestMain(m) }
	m.Run()
}

func TestUncovered(t *testing.T) {} // want "test function TestUncovered is not covered by goleak"
