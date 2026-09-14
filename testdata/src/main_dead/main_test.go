package main_dead

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	if false {
		goleak.VerifyTestMain(m)
	}
	m.Run()
}

func TestUncovered(t *testing.T) {} // want "test function TestUncovered is not covered by goleak"
