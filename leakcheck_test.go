package leakcheck_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/rleungx/leakcheck"
	"golang.org/x/tools/go/analysis"
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

func TestExcludeMultipleFiles(t *testing.T) {
	config := &leakcheck.Config{
		ExcludeFiles: "exclude_file_a_test.go,exclude_file_b_test.go",
	}
	analyzer := leakcheck.NewWithConfig(config)
	testdata := analysistest.TestData()
	// Should only report issues for normal_test.go
	// exclude_file_a_test.go and exclude_file_b_test.go should be ignored
	analysistest.Run(t, testdata, analyzer, "exclude_multiple_files")
}

func TestExcludeMultiplePackages(t *testing.T) {
	config := &leakcheck.Config{
		ExcludePackages: "exclude_package_a,exclude_package_b",
	}
	analyzer := leakcheck.NewWithConfig(config)
	testdata := analysistest.TestData()
	// Should not report any issues since both exclude_package_a and exclude_package_b are excluded
	analysistest.Run(t, testdata, analyzer, "exclude_multiple_packages/exclude_package_a", "exclude_multiple_packages/exclude_package_b", "exclude_multiple_packages/normal_package")
}

func TestAlias(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, leakcheck.Analyzer, "alias")
}

func TestAliasMain(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, leakcheck.Analyzer, "alias_main")
}

func TestNoImportIgnoresNonTestFiles(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, leakcheck.Analyzer, "no_import_non_test_file")
}

func TestNestedDeferDoesNotCountAsCoverage(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, leakcheck.Analyzer, "nested_defer")
}

func BenchmarkAnalyzer(b *testing.B) {
	b.Run("NoImport", func(b *testing.B) {
		analyzer := leakcheck.NewWithConfig(&leakcheck.Config{
			ExcludePackages: "vendor,third_party",
			ExcludeFiles:    "mock_*,zz_generated*.go,legacy_.*",
			Concurrency:     8,
		})
		pass := mustBenchmarkPass(b, generateNoImportSources(48, 16))
		verifyBenchmarkReports(b, analyzer.Run, pass, 48*16)
		pass.Report = func(analysis.Diagnostic) {}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := analyzer.Run(pass); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("MixedCoverage", func(b *testing.B) {
		analyzer := leakcheck.NewWithConfig(&leakcheck.Config{Concurrency: 8})
		pass := mustBenchmarkPass(b, generateMixedCoverageSources(40, 18))
		verifyBenchmarkReports(b, analyzer.Run, pass, 40*9)
		pass.Report = func(analysis.Diagnostic) {}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := analyzer.Run(pass); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("ExcludedFiles", func(b *testing.B) {
		analyzer := leakcheck.NewWithConfig(&leakcheck.Config{
			ExcludeFiles: "mock_*,zz_generated*.go,legacy_.*",
			Concurrency:  8,
		})
		pass := mustBenchmarkPass(b, generateExcludedFileSources(60, 12))
		verifyBenchmarkReports(b, analyzer.Run, pass, 30*12)
		pass.Report = func(analysis.Diagnostic) {}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := analyzer.Run(pass); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func mustBenchmarkPass(b *testing.B, sources map[string]string) *analysis.Pass {
	b.Helper()

	fset := token.NewFileSet()
	files := make([]string, 0, len(sources))
	for name := range sources {
		files = append(files, name)
	}
	sort.Strings(files)

	parsed := make([]*ast.File, 0, len(files))
	for _, name := range files {
		filename := filepath.Join("benchmark", name)
		file, err := parser.ParseFile(fset, filename, sources[name], 0)
		if err != nil {
			b.Fatalf("parse %s: %v", name, err)
		}
		parsed = append(parsed, file)
	}

	return &analysis.Pass{
		Fset:   fset,
		Files:  parsed,
		Pkg:    types.NewPackage("example.com/benchmark", "benchmark"),
		Report: func(analysis.Diagnostic) {},
	}
}

func verifyBenchmarkReports(b *testing.B, runner func(*analysis.Pass) (interface{}, error), pass *analysis.Pass, want int) {
	b.Helper()

	got := 0
	pass.Report = func(analysis.Diagnostic) { got++ }
	if _, err := runner(pass); err != nil {
		b.Fatalf("warmup run failed: %v", err)
	}
	if got != want {
		b.Fatalf("unexpected reports: got %d want %d", got, want)
	}
}

func generateNoImportSources(fileCount, testsPerFile int) map[string]string {
	sources := make(map[string]string, fileCount)
	for fileIndex := 0; fileIndex < fileCount; fileIndex++ {
		var sb strings.Builder
		sb.WriteString("package benchmark\n\nimport \"testing\"\n\n")
		for testIndex := 0; testIndex < testsPerFile; testIndex++ {
			fmt.Fprintf(&sb, "func TestCase%d_%d(t *testing.T) {\n\t_ = t\n}\n\n", fileIndex, testIndex)
		}
		sources[fmt.Sprintf("case_%02d_test.go", fileIndex)] = sb.String()
	}
	return sources
}

func generateMixedCoverageSources(fileCount, testsPerFile int) map[string]string {
	sources := make(map[string]string, fileCount)
	for fileIndex := 0; fileIndex < fileCount; fileIndex++ {
		var sb strings.Builder
		sb.WriteString("package benchmark\n\nimport (\n\t\"testing\"\n\n\t\"go.uber.org/goleak\"\n)\n\n")
		for testIndex := 0; testIndex < testsPerFile; testIndex++ {
			fmt.Fprintf(&sb, "func TestCase%d_%d(t *testing.T) {\n", fileIndex, testIndex)
			if testIndex%2 == 0 {
				sb.WriteString("\tdefer goleak.VerifyNone(t)\n")
			}
			sb.WriteString("\t_ = goleak.IgnoreTopFunction\n")
			sb.WriteString("}\n\n")
		}
		sources[fmt.Sprintf("mixed_%02d_test.go", fileIndex)] = sb.String()
	}
	return sources
}

func generateExcludedFileSources(fileCount, testsPerFile int) map[string]string {
	sources := make(map[string]string, fileCount)
	for fileIndex := 0; fileIndex < fileCount; fileIndex++ {
		var sb strings.Builder
		sb.WriteString("package benchmark\n\nimport \"testing\"\n\n")
		for testIndex := 0; testIndex < testsPerFile; testIndex++ {
			fmt.Fprintf(&sb, "func TestCase%d_%d(t *testing.T) {\n\t_ = t\n}\n\n", fileIndex, testIndex)
		}

		name := fmt.Sprintf("normal_%02d_test.go", fileIndex)
		if fileIndex%2 == 0 {
			name = fmt.Sprintf("mock_%02d_test.go", fileIndex)
		}
		sources[name] = sb.String()
	}
	return sources
}
