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
