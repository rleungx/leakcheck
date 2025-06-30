package multiple_files_with_main

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain 在第三个文件中 - 应该覆盖整个包的所有测试
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
