package alias

import (
	"testing"

	gl "go.uber.org/goleak"
)

func TestWithAlias(t *testing.T) { // want "test function TestWithAlias is not covered by goleak \\(missing defer goleak.VerifyNone\\(t\\)\\)"
	// This test function doesn't have defer gl.VerifyNone(t)
}

func TestWithAliasCorrect(t *testing.T) {
	defer gl.VerifyNone(t)
	// This test function is correctly covered
}
