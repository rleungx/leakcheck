package main_without_verify

import (
	"testing"

	"go.uber.org/goleak"
)

// Test with TestMain that doesn't call goleak.VerifyTestMain - should trigger warning

func TestWithoutVerify(t *testing.T) { // want "test function TestWithoutVerify is not covered by goleak \\(TestMain exists but doesn't call goleak.VerifyTestMain\\)"
	// test logic here
}

func TestAnotherWithoutVerify(t *testing.T) { // want "test function TestAnotherWithoutVerify is not covered by goleak \\(TestMain exists but doesn't call goleak.VerifyTestMain\\)"
	// test logic here
}

// TestMain exists but doesn't call goleak.VerifyTestMain
func TestMain(m *testing.M) {
	// Missing goleak.VerifyTestMain(m)
	// Just to use the import, we'll reference it in a comment: goleak
	_ = goleak.IgnoreTopFunction // use the import to avoid "imported and not used" error
	m.Run()
}
