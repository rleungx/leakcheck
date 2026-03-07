package no_import_non_test_file

import "testing"

func TestHelper(t *testing.T) {
	// This lives in a non-test file and must be ignored.
}
