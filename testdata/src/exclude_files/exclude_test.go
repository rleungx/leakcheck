package exclude_files

import (
	"testing"
)

// Test without goleak import - this file will be excluded by pattern
func TestExcludeFile(t *testing.T) {
	// test logic here - no want comment because this file should be excluded
}
