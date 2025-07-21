package normal_package

import "testing"

func TestNormalPackage(t *testing.T) { // want "test function TestNormalPackage is not covered by goleak \\(goleak not imported\\)"
	// Missing goleak.VerifyNone(t)
}
