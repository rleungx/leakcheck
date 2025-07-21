package exclude_multiple_files

import (
	"testing"
)

// Test without goleak import - this file may be excluded depending on the test
func TestExcludeFileB(t *testing.T) {
	// test logic here
}

func TestAnotherExcludeFileB(t *testing.T) {
	// test logic here
}
