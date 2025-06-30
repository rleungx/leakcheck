package multiple_files

import (
	"testing"

	"go.uber.org/goleak"
)

// 第一个文件中的测试 - 有 goleak 覆盖
func TestFirstFileWithGoleak(t *testing.T) {
	defer goleak.VerifyNone(t)
	// test logic here
}

// 第一个文件中的测试 - 没有 goleak 覆盖
func TestFirstFileWithoutGoleak(t *testing.T) { // want "test function TestFirstFileWithoutGoleak is not covered by goleak \\(missing defer goleak.VerifyNone\\(t\\)\\)"
	// test logic here
}
