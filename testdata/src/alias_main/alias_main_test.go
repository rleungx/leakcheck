package alias_main

import (
	"testing"

	gl "go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	gl.VerifyTestMain(m)
}

func TestWithMainAlias(t *testing.T) {
	// This test is covered by TestMain with alias
}
