package main_deferred_exit

import (
	"os"
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	defer goleak.VerifyTestMain(m)
	os.Exit(m.Run())
}

func TestUncovered(t *testing.T) {} // want "test function TestUncovered is not covered by goleak"
