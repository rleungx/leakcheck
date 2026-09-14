package main_both_branches

import (
	"os"
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	if os.Getenv("CUSTOM") != "" {
		goleak.VerifyTestMain(m, goleak.IgnoreCurrent())
	} else {
		goleak.VerifyTestMain(m)
	}
}

func TestCovered(t *testing.T) {}
