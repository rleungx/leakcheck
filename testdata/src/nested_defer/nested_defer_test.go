package nested_defer

import (
	"testing"

	"go.uber.org/goleak"
)

func TestNestedDefer(t *testing.T) { // want "test function TestNestedDefer is not covered by goleak \\(missing defer goleak.VerifyNone\\(t\\)\\)"
	func() {
		defer goleak.VerifyNone(t)
	}()
}
