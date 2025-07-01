package leakcheck_test

import (
	"testing"

	"github.com/rleungx/leakcheck"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestBasic(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, leakcheck.Analyzer, "basic")
}

func TestNoImport(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, leakcheck.Analyzer, "no_import")
}

func TestMainWithVerify(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, leakcheck.Analyzer, "main_with_verify")
}

func TestMainWithoutVerify(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, leakcheck.Analyzer, "main_without_verify")
}

func TestMultipleFiles(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, leakcheck.Analyzer, "multiple_files")
}

func TestMultipleFilesWithMain(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, leakcheck.Analyzer, "multiple_files_with_main")
}

func TestExcludePackages(t *testing.T) {
	config := &leakcheck.Config{
		ExcludePackages: "exclude_package",
	}
	analyzer := leakcheck.NewWithConfig(config)
	testdata := analysistest.TestData()
	// Should not report any issues since exclude_package package is excluded
	analysistest.Run(t, testdata, analyzer, "exclude_package")
}

func TestExcludePackagesRegex(t *testing.T) {
	config := &leakcheck.Config{
		ExcludePackages: ".*exclude.*",
	}
	analyzer := leakcheck.NewWithConfig(config)
	testdata := analysistest.TestData()
	// Should not report any issues since packages matching .*exclude.* are excluded
	analysistest.Run(t, testdata, analyzer, "exclude_package")
}

func TestExcludeFiles(t *testing.T) {
	config := &leakcheck.Config{
		ExcludeFiles: "exclude_test.go",
	}
	analyzer := leakcheck.NewWithConfig(config)
	testdata := analysistest.TestData()
	// Should only report issues for normal_test.go, exclude_test.go should be ignored
	analysistest.Run(t, testdata, analyzer, "exclude_files")
}

func TestExcludeFilesRegex(t *testing.T) {
	config := &leakcheck.Config{
		ExcludeFiles: "exclude_test\\.go$",
	}
	analyzer := leakcheck.NewWithConfig(config)
	testdata := analysistest.TestData()
	// Should only report issues for normal_test.go, exclude_test.go should be ignored
	analysistest.Run(t, testdata, analyzer, "exclude_files")
}

func TestAlias(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, leakcheck.Analyzer, "alias")
}

func TestAliasMain(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, leakcheck.Analyzer, "alias_main")
}
