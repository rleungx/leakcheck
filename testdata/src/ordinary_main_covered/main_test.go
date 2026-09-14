package ordinary_main_covered

import (
	. "testing"

	"go.uber.org/goleak"
)

func TestMain(t *T) {
	defer goleak.VerifyNone(t)
}

// An ordinary test named TestMain does not cover its siblings.
func TestSibling(t *T) {} // want "test function TestSibling is not covered by goleak"

type helper struct{}

func (*helper) TestMain(t *T) {}
