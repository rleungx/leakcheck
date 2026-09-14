package multiple_aliases

import (
	"testing"

	"go.uber.org/goleak"
)

func TestDefaultName(t *testing.T) {
	defer goleak.VerifyNone(t)
}
