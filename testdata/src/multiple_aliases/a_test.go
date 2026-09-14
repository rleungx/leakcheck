package multiple_aliases

import (
	"testing"

	gl "go.uber.org/goleak"
)

func TestAlias(t *testing.T) {
	defer gl.VerifyNone(t)
}
