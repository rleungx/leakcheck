package multiple_files

import (
	"testing"

	"go.uber.org/goleak"
)

// 第三个文件中的测试 - 没有 goleak 覆盖
func TestThirdFileWithoutGoleak(t *testing.T) { // want "test function TestThirdFileWithoutGoleak is not covered by goleak \\(missing defer goleak.VerifyNone\\(t\\)\\)"
	helperFunction()
}

// Helper test function (not a test, should be ignored)
func helperFunction() {
	// This should not be reported
	// Use goleak import to avoid "imported and not used" error
	_ = goleak.IgnoreTopFunction
}
