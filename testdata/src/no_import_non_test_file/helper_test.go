package no_import_non_test_file

import "testing"

func TestReal(t *testing.T) { // want "test function TestReal is not covered by goleak \\(goleak not imported\\)"
	// Missing goleak import on purpose.
}
