package no_import

import (
	"testing"
)

// Test without goleak import - should trigger warning
func TestWithoutGoleakImport(t *testing.T) { // want "test function TestWithoutGoleakImport is not covered by goleak \\(goleak not imported\\)"
	// test logic here
}

func TestAnotherWithoutImport(t *testing.T) { // want "test function TestAnotherWithoutImport is not covered by goleak \\(goleak not imported\\)"
	// test logic here
}
