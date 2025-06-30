package main_with_verify

import (
	"testing"

	"go.uber.org/goleak"
)

// Test with goleak.VerifyTestMain - should not trigger warning
func TestWithVerify(t *testing.T) {
	// This should be covered by TestMain
}

func TestAnotherWithVerify(t *testing.T) {
	// This should also be covered by TestMain
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
