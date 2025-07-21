package exclude_package_b

import "testing"

// This test should be excluded, so no want comment needed
func TestExcludePackageB(t *testing.T) {
	// Missing goleak.VerifyNone(t)
}
