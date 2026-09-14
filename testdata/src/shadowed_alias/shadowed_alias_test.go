package shadowed_alias

import (
	"testing"

	gl "go.uber.org/goleak"
)

var _ = gl.IgnoreTopFunction

type fakeGoleak struct{}

func (fakeGoleak) VerifyNone(*testing.T) {}

func TestShadowedAlias(t *testing.T) { // want "test function TestShadowedAlias is not covered by goleak \\(missing defer goleak.VerifyNone\\(t\\)\\)"
	gl := fakeGoleak{}
	defer gl.VerifyNone(t)
}
