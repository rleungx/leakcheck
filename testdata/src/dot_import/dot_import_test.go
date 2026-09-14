package dot_import

import (
	"testing"

	. "go.uber.org/goleak"
)

func TestDotImport(t *testing.T) {
	defer VerifyNone(t)
}
