package exclude_test

import (
	"testing"
)

// Test without goleak import - but this package will be excluded
func TestExclude(t *testing.T) {
	// test logic here - no want comment because this should be excluded
}

func TestAnotherExclude(t *testing.T) {
	// test logic here - no want comment because this should be excluded
}
