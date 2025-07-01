package exclude_files

import (
	"testing"
)

// Test without goleak import - this file will NOT be excluded
func TestNormalFile(t *testing.T) { // want "test function TestNormalFile is not covered by goleak \\(goleak not imported\\)"
	// test logic here
}
