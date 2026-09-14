package main_deferred

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	defer goleak.VerifyTestMain(m)
}

func TestCovered(t *testing.T) {}
