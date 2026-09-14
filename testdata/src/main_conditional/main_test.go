package main_conditional

import (
	"os"
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	if os.Getenv("CHECK_LEAKS") != "" {
		goleak.VerifyTestMain(m)
	} else {
		m.Run()
	}
}

func TestUncovered(t *testing.T) {} // want "test function TestUncovered is not covered by goleak"
