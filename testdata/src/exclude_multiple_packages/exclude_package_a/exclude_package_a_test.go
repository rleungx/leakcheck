package exclude_package_a

import "testing"

// This test should be excluded, so no want comment needed
func TestExcludePackageA(t *testing.T) {
	// Missing goleak.VerifyNone(t)
}
