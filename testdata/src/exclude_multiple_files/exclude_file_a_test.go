package exclude_multiple_files

import (
	"testing"
)

// Test without goleak import - this file will be excluded by pattern
func TestExcludeFileA(t *testing.T) {
	// test logic here - no want comment because this file should be excluded
}

func TestAnotherExcludeFileA(t *testing.T) {
	// test logic here - no want comment because this file should be excluded
}
