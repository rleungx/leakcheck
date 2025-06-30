package multiple_files_with_main

import (
	"testing"

	"go.uber.org/goleak"
)

// 第二个文件中的测试 - 也应该被 TestMain 覆盖
func TestFileTwoWithMain(t *testing.T) {
	// test logic here - covered by TestMain
	// Use goleak import to avoid "imported and not used" error
	_ = goleak.IgnoreTopFunction
}

func TestYetAnotherTest(t *testing.T) {
	// test logic here - also covered by TestMain
}
