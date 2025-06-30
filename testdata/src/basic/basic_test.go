package basic

import (
	"testing"

	"go.uber.org/goleak"
)

// Test function with proper goleak coverage - should not trigger warning
func TestWithGoleak(t *testing.T) {
	defer goleak.VerifyNone(t)
	// test logic here
}

// Test function without goleak coverage - should trigger warning
func TestWithoutGoleak(t *testing.T) { // want "test function TestWithoutGoleak is not covered by goleak \\(missing defer goleak.VerifyNone\\(t\\)\\)"
	// test logic here
}
