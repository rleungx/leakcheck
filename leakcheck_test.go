package leakcheck

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestBasic(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "basic")
}

func TestNoImport(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "no_import")
}

func TestMainWithVerify(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "main_with_verify")
}

func TestMainWithoutVerify(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "main_without_verify")
}

func TestMultipleFiles(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "multiple_files")
}

func TestMultipleFilesWithMain(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "multiple_files_with_main")
}

func TestExcludePackages(t *testing.T) {
	config := &Config{
		ExcludePackages: "exclude_package",
	}
	analyzer := NewWithConfig(config)
	testdata := analysistest.TestData()
	// Should not report any issues since exclude_package package is excluded
	analysistest.Run(t, testdata, analyzer, "exclude_package")
}

func TestExcludePackagesRegex(t *testing.T) {
	config := &Config{
		ExcludePackages: ".*exclude.*",
	}
	analyzer := NewWithConfig(config)
	testdata := analysistest.TestData()
	// Should not report any issues since packages matching .*exclude.* are excluded
	analysistest.Run(t, testdata, analyzer, "exclude_package")
}

func TestExcludeFiles(t *testing.T) {
	config := &Config{
		ExcludeFiles: "*exclude_test.go",
	}
	analyzer := NewWithConfig(config)
	testdata := analysistest.TestData()
	// Should only report issues for normal_test.go, exclude_test.go should be ignored
	analysistest.Run(t, testdata, analyzer, "exclude_files")
}
