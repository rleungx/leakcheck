package leakcheck

import (
	"go/ast"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Config holds the configuration for the leakcheck analyzer
type Config struct {
	ExcludePackages string
	ExcludeFiles    string
}

// New creates a new leakcheck analyzer with default configuration
func New() *analysis.Analyzer {
	return NewWithConfig(&Config{})
}

// NewWithConfig creates a new leakcheck analyzer with custom configuration
func NewWithConfig(config *Config) *analysis.Analyzer {
	analyzer := &analysis.Analyzer{
		Name:     "leakcheck",
		Doc:      "check that all tests are covered by goleak",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Run:      run(config),
	}

	// Add flags for command-line usage
	analyzer.Flags.StringVar(&config.ExcludePackages, "exclude-packages", config.ExcludePackages, "comma-separated list of package patterns to exclude (supports regex)")
	analyzer.Flags.StringVar(&config.ExcludeFiles, "exclude-files", config.ExcludeFiles, "comma-separated list of file patterns to exclude (supports regex)")

	return analyzer
}

// Analyzer is the default analyzer instance for backward compatibility
var Analyzer = New()

// run creates a run function with the given configuration
func run(config *Config) func(*analysis.Pass) (interface{}, error) {
	return func(pass *analysis.Pass) (interface{}, error) {
		inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

		// Check if we have any files to analyze
		if len(pass.Files) == 0 {
			return nil, nil
		}

		// Check if package should be excluded
		if shouldExcludePackage(pass.Pkg.Path(), config) {
			return nil, nil
		}

		// Check if we have any non-excluded test files
		if !hasNonExcludedTestFiles(pass, config) {
			return nil, nil
		}

		var (
			hasTestMain         bool
			hasVerifyTestMain   bool
			testFuncs           []string
			funcsCoveredByDefer map[string]bool = make(map[string]bool)
		)

		// Check if goleak is imported and get its alias
		goleakAlias := getGoleakAlias(pass.Files)
		hasGoleakImport := goleakAlias != ""

		// If no goleak import, report for all test functions
		if !hasGoleakImport {
			inspect.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node) {
				fd := n.(*ast.FuncDecl)
				if isTestFunction(fd.Name.Name) {
					// Check if the file containing this function should be excluded
					pos := pass.Fset.Position(fd.Pos())
					if !shouldExcludeFile(pos.Filename, config) {
						pass.Reportf(fd.Pos(), "test function %s is not covered by goleak (goleak not imported)", fd.Name.Name)
					}
				}
			})
			return nil, nil
		}

		// Find TestMain and test functions
		inspect.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node) {
			fd := n.(*ast.FuncDecl)

			if fd.Name.Name == "TestMain" {
				hasTestMain = true
				// Check if TestMain calls goleak.VerifyTestMain
				ast.Inspect(fd, func(node ast.Node) bool {
					if call, ok := node.(*ast.CallExpr); ok {
						if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
							if isGoleakCall(sel, "VerifyTestMain", goleakAlias) {
								hasVerifyTestMain = true
							}
						}
					}
					return true
				})
			} else if isTestFunction(fd.Name.Name) {
				testFuncs = append(testFuncs, fd.Name.Name)

				// Check if this test function has defer goleak.VerifyNone
				ast.Inspect(fd, func(node ast.Node) bool {
					if defer_stmt, ok := node.(*ast.DeferStmt); ok {
						if call, ok := defer_stmt.Call.Fun.(*ast.SelectorExpr); ok {
							if isGoleakCall(call, "VerifyNone", goleakAlias) {
								funcsCoveredByDefer[fd.Name.Name] = true
							}
						}
					}
					return true
				})
			}
		})

		// Report issues
		if hasTestMain && hasVerifyTestMain {
			// If TestMain with VerifyTestMain exists, all tests are covered
			return nil, nil
		}

		// Check individual test functions
		for _, testFunc := range testFuncs {
			if !funcsCoveredByDefer[testFunc] {
				// Find the function declaration to report at its position
				inspect.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node) {
					fd := n.(*ast.FuncDecl)
					if fd.Name.Name == testFunc {
						// Check if the file containing this function should be excluded
						pos := pass.Fset.Position(fd.Pos())
						if !shouldExcludeFile(pos.Filename, config) {
							if hasTestMain && !hasVerifyTestMain {
								pass.Reportf(fd.Pos(), "test function %s is not covered by goleak (TestMain exists but doesn't call goleak.VerifyTestMain)", testFunc)
							} else {
								pass.Reportf(fd.Pos(), "test function %s is not covered by goleak (missing defer goleak.VerifyNone(t))", testFunc)
							}
						}
					}
				})
			}
		}

		return nil, nil
	}
}

// isTestFile checks if the filename indicates a test file
func isTestFile(filename string) bool {
	return strings.HasSuffix(filename, "_test.go")
}

// isTestFunction checks if a function name is a test function
func isTestFunction(name string) bool {
	return strings.HasPrefix(name, "Test") && name != "TestMain"
}

// isGoleakCall checks if a selector expression is a call to goleak with the specified method
func isGoleakCall(sel *ast.SelectorExpr, method, alias string) bool {
	if sel.Sel.Name != method {
		return false
	}

	if ident, ok := sel.X.(*ast.Ident); ok {
		return ident.Name == alias
	}

	return false
}

// GetGoleakAlias checks if any file imports goleak and returns its alias/name (exported for testing)
func GetGoleakAlias(files []*ast.File) string {
	return getGoleakAlias(files)
}

// getGoleakAlias checks if any file imports goleak and returns its alias/name
func getGoleakAlias(files []*ast.File) string {
	for _, file := range files {
		for _, imp := range file.Imports {
			if imp.Path != nil && (imp.Path.Value == `"go.uber.org/goleak"` || imp.Path.Value == `"github.com/uber-go/goleak"`) {
				if imp.Name != nil {
					// Has explicit alias
					return imp.Name.Name
				}
				// Default import name
				return "goleak"
			}
		}
	}
	return ""
}

// shouldExcludePackage checks if a package should be excluded based on patterns
func shouldExcludePackage(pkgPath string, config *Config) bool {
	return matchesAnyPattern(pkgPath, config.ExcludePackages)
}

// shouldExcludeFile checks if a file should be excluded based on patterns
func shouldExcludeFile(filename string, config *Config) bool {
	return matchesAnyPattern(filename, config.ExcludeFiles)
}

// matchesAnyPattern checks if a string matches any of the comma-separated patterns
func matchesAnyPattern(str, patterns string) bool {
	if patterns == "" {
		return false
	}

	for _, pattern := range strings.Split(patterns, ",") {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}

		// Try exact match first
		if str == pattern || strings.HasSuffix(str, pattern) {
			return true
		}

		// Try regex match
		if matched, err := regexp.MatchString(pattern, str); err == nil && matched {
			return true
		}

		// Try glob-like pattern (simple * wildcard)
		if strings.Contains(pattern, "*") {
			globPattern := strings.ReplaceAll(pattern, "*", ".*")
			if matched, err := regexp.MatchString("^"+globPattern+"$", str); err == nil && matched {
				return true
			}
		}
	}

	return false
}

// hasNonExcludedTestFiles checks if there are any test files that are not excluded
func hasNonExcludedTestFiles(pass *analysis.Pass, config *Config) bool {
	for _, file := range pass.Files {
		filename := pass.Fset.Position(file.Pos()).Filename
		if isTestFile(filename) && !shouldExcludeFile(filename, config) {
			return true
		}
	}
	return false
}
