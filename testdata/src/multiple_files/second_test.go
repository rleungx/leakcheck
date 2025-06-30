package multiple_files

import (
	"testing"

	"go.uber.org/goleak"
)

// 第二个文件中的测试 - 有 goleak 覆盖
func TestSecondFileWithGoleak(t *testing.T) {
	defer goleak.VerifyNone(t)
	// test logic here
}

// 第二个文件中的测试 - 没有 goleak 覆盖
func TestSecondFileWithoutGoleak(t *testing.T) { // want "test function TestSecondFileWithoutGoleak is not covered by goleak \\(missing defer goleak.VerifyNone\\(t\\)\\)"
	// test logic here
}
