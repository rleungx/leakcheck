package exclude_test

import (
	"testing"
)

// Test without goleak import - but this package will be excluded
func TestWithoutGoleakImport(t *testing.T) {
	// test logic here - no want comment because this should be excluded
}

func TestAnotherWithoutImport(t *testing.T) {
	// test logic here - no want comment because this should be excluded
}
