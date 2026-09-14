package leakcheck

import (
	"testing"

	"go.uber.org/goleak"
)

func TestPatternSyntax(t *testing.T) {
	defer goleak.VerifyNone(t)

	for _, tc := range []struct {
		pattern string
		input   string
		want    bool
	}{
		{"mock_*", "mock_helpers_test.go", true},
		{"mock_*", "real_mock_test.go", false},
		{"mock_*", "/project/mock_helpers/normal_test.go", false},
		{"mock_*.go", "mock_helpers_test.go", true},
		{"mock_*.go", "mock_helpers_testXgo", false},
		{"*_mock_test.go", "real_mock_test.go", true},
		{"*_mock_test.go", "real_mock_testXgo", false},
		{"*_mock_test.go", "normal_test.go", false},
		{"*.*", "file.go", true},
		{"*.*", "filename", false},
		{"mock_*.*", "mock_helpers.go", true},
		{"mock_*.*", "real_mock_helpers.go", false},
		{"dir/*.*", "dir/file.go", true},
		{"dir/*.*", "dir/filename", false},
		{"legacy_.*", "legacy_helpers_test.go", true},
		{"legacy_.*", "normal_test.go", false},
		{".*mock.*", "real_mock_test.go", true},
		{`^mock_.*\.go$`, "mock_helpers_test.go", true},
		{`^mock_.*\.go$`, "real_mock_helpers_test.go", false},
		{"vendor", "project/vendor/dependency", true},
	} {
		t.Run(tc.pattern+"/"+tc.input, func(t *testing.T) {
			if got := matchesPattern(tc.input, tc.pattern); got != tc.want {
				t.Errorf("matchesPattern(%q, %q) = %v, want %v", tc.input, tc.pattern, got, tc.want)
			}
		})
	}
}
