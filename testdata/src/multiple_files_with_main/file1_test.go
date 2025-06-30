package multiple_files_with_main

import (
	"testing"

	"go.uber.org/goleak"
)

// 第一个文件中的测试 - 应该被 TestMain 覆盖，不应该报告问题
func TestFileOneWithMain(t *testing.T) {
	// test logic here - covered by TestMain
	// Use goleak import to avoid "imported and not used" error
	_ = goleak.IgnoreTopFunction
}

func TestAnotherInFileOne(t *testing.T) {
	// test logic here - also covered by TestMain
}
