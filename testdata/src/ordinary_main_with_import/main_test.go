package ordinary_main_with_import

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(t *testing.T) { // want "test function TestMain is not covered by goleak"
	_ = goleak.VerifyNone
}

func TestCovered(t *testing.T) {
	defer goleak.VerifyNone(t)
}
