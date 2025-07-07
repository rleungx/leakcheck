package leakcheck

import (
	"go/ast"
	"go/token"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Config holds the configuration for the leakcheck analyzer
type Config struct {
	ExcludePackages string
	ExcludeFiles    string
}

// regexCache caches compiled regular expressions for better performance
var (
	regexCache = make(map[string]*regexp.Regexp)
	regexMutex sync.RWMutex
)

// New creates a new leakcheck analyzer with default configuration
func New() *analysis.Analyzer {
	return NewWithConfig(&Config{})
}

// NewWithConfig creates a new leakcheck analyzer with custom configuration
func NewWithConfig(config *Config) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:     "leakcheck",
		Doc:      "check that all tests are covered by goleak",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Run:      run(config),
	}
}

// Analyzer is the default analyzer instance for backward compatibility
var Analyzer = New()

// run creates a run function with the given configuration
func run(config *Config) func(*analysis.Pass) (interface{}, error) {
	return func(pass *analysis.Pass) (interface{}, error) {
		inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

		// Early bailout checks for performance
		if len(pass.Files) == 0 {
			return nil, nil
		}

		// Check if package should be excluded first (fastest check)
		if shouldExcludePackage(pass.Pkg.Path(), config) {
			return nil, nil
		}

		// Check if we have any non-excluded test files
		if !hasNonExcludedTestFiles(pass, config) {
			return nil, nil
		}

		// Check if goleak is imported and get its alias
		goleakAlias := getGoleakAlias(pass.Files)

		// If no goleak import, report for all test functions
		if goleakAlias == "" {
			reportUncoveredTestFunctions(pass, inspect, config, "goleak not imported")
			return nil, nil
		}

		// Analyze test functions to collect information in a single traversal
		result := analyzeTestFunctions(inspect, pass, goleakAlias)

		// Report issues
		if result.hasTestMain && result.hasVerifyTestMain {
			// If TestMain with VerifyTestMain exists, all tests are covered
			return nil, nil
		}

		// Check individual test functions
		for _, testFunc := range result.testFuncs {
			if !result.funcsCoveredByDefer[testFunc.name] {
				reason := "missing defer goleak.VerifyNone(t)"
				if result.hasTestMain && !result.hasVerifyTestMain {
					reason = "TestMain exists but doesn't call goleak.VerifyTestMain"
				}
				// Report directly using cached position info
				if !shouldExcludeFile(testFunc.filename, config) {
					pass.Reportf(testFunc.pos, "test function %s is not covered by goleak (%s)", testFunc.name, reason)
				}
			}
		}

		return nil, nil
	}
}

// analysisResult holds the analysis results to avoid multiple traversals
type analysisResult struct {
	hasTestMain         bool
	hasVerifyTestMain   bool
	testFuncs           []testFuncInfo
	funcsCoveredByDefer map[string]bool
}

// testFuncInfo holds information about a test function
type testFuncInfo struct {
	name     string
	pos      token.Pos
	filename string
}

// analyzeTestFunctions performs a single traversal to collect all test function information
func analyzeTestFunctions(inspect *inspector.Inspector, pass *analysis.Pass, goleakAlias string) *analysisResult {
	result := &analysisResult{
		funcsCoveredByDefer: make(map[string]bool),
	}

	// Use a more efficient traversal that checks multiple node types in one pass
	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil), (*ast.DeferStmt)(nil), (*ast.CallExpr)(nil)}

	var currentTestFunc string
	var inTestMain bool

	inspect.Preorder(nodeFilter, func(n ast.Node) {
		switch node := n.(type) {
		case *ast.FuncDecl:
			funcName := node.Name.Name
			currentTestFunc = ""
			inTestMain = false

			if funcName == testMainFunc {
				result.hasTestMain = true
				inTestMain = true
			} else if isTestFunction(funcName) {
				currentTestFunc = funcName
				pos := pass.Fset.Position(node.Pos())
				testFunc := testFuncInfo{
					name:     funcName,
					pos:      node.Pos(),
					filename: pos.Filename,
				}
				result.testFuncs = append(result.testFuncs, testFunc)
			}

		case *ast.CallExpr:
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
				if inTestMain && isGoleakCall(sel, verifyTestMain, goleakAlias) {
					result.hasVerifyTestMain = true
				}
			}

		case *ast.DeferStmt:
			if currentTestFunc != "" {
				if call, ok := node.Call.Fun.(*ast.SelectorExpr); ok {
					if isGoleakCall(call, verifyNone, goleakAlias) {
						result.funcsCoveredByDefer[currentTestFunc] = true
					}
				}
			}
		}
	})

	return result
}

// Constants for goleak package paths and method names
const (
	goleakUberPath   = `"go.uber.org/goleak"`
	goleakGithubPath = `"github.com/uber-go/goleak"`
	defaultAlias     = "goleak"
	verifyTestMain   = "VerifyTestMain"
	verifyNone       = "VerifyNone"
	testPrefix       = "Test"
	testMainFunc     = "TestMain"
	testFileSuffix   = "_test.go"
)

// isTestFile checks if the filename indicates a test file
func isTestFile(filename string) bool {
	return strings.HasSuffix(filename, testFileSuffix)
}

// isTestFunction checks if a function name is a test function
func isTestFunction(name string) bool {
	return strings.HasPrefix(name, testPrefix) && name != testMainFunc
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

// getGoleakAlias checks if any file imports goleak and returns its alias/name
func getGoleakAlias(files []*ast.File) string {
	for _, file := range files {
		for _, imp := range file.Imports {
			if imp.Path != nil && (imp.Path.Value == goleakUberPath || imp.Path.Value == goleakGithubPath) {
				if imp.Name != nil {
					return imp.Name.Name
				}
				return defaultAlias
			}
		}
	}
	return ""
}

// shouldExcludePackage checks if a package should be excluded
func shouldExcludePackage(pkgPath string, config *Config) bool {
	if config.ExcludePackages == "" {
		return false
	}
	return matchesAnyPattern(pkgPath, config.ExcludePackages)
}

// shouldExcludeFile checks if a file should be excluded
func shouldExcludeFile(filename string, config *Config) bool {
	if config.ExcludeFiles == "" {
		return false
	}
	return matchesAnyPattern(filename, config.ExcludeFiles)
}

// matchesAnyPattern checks if a string matches any of the comma-separated patterns
func matchesAnyPattern(str, patterns string) bool {
	if patterns == "" {
		return false
	}

	// Avoid creating string slice if only one pattern
	if !strings.Contains(patterns, ",") {
		return matchesPattern(str, strings.TrimSpace(patterns))
	}

	for _, pattern := range strings.Split(patterns, ",") {
		pattern = strings.TrimSpace(pattern)
		if pattern != "" && matchesPattern(str, pattern) {
			return true
		}
	}

	return false
}

// matchesPattern checks if a string matches a single pattern
// This function is optimized for performance with large projects by using:
// 1. Fast path for exact matches
// 2. Fast path for substring matches (common for package exclusions)
// 3. Fast path for simple suffix matches (common for file exclusions)
// 4. Cached regex compilation for complex patterns
func matchesPattern(str, pattern string) bool {
	// Fast path: exact match
	if str == pattern {
		return true
	}

	// Fast path: pattern as substring (common case for package exclusions)
	if strings.Contains(str, pattern) {
		return true
	}

	// Fast path: simple suffix match (common case for file patterns)
	if !containsRegexMetachars(pattern) && !strings.Contains(pattern, "*") {
		return strings.HasSuffix(str, pattern)
	}

	// Handle simple glob patterns (only convert if it looks like a simple glob)
	// If the pattern contains regex metacharacters other than *, treat it as regex
	if strings.Contains(pattern, "*") && !containsRegexMetachars(pattern) {
		pattern = strings.ReplaceAll(pattern, "*", ".*")
		pattern = "^" + pattern + "$"
	}

	// Try regex match with caching
	regexMutex.RLock()
	re, ok := regexCache[pattern]
	regexMutex.RUnlock()
	if !ok {
		var err error
		re, err = regexp.Compile(pattern)
		if err != nil {
			return false
		}
		regexMutex.Lock()
		regexCache[pattern] = re
		regexMutex.Unlock()
	}

	return re.MatchString(str)
}

// containsRegexMetachars checks if pattern contains regex metacharacters other than *
func containsRegexMetachars(pattern string) bool {
	// Use faster byte-based check with optimized switch
	for _, r := range pattern {
		switch r {
		case '.', '^', '$', '+', '?', '[', ']', '(', ')', '{', '}', '|', '\\':
			return true
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

// reportUncoveredTestFunctions reports all test functions that are not covered
func reportUncoveredTestFunctions(pass *analysis.Pass, inspect *inspector.Inspector, config *Config, reason string) {
	inspect.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node) {
		fd := n.(*ast.FuncDecl)
		if isTestFunction(fd.Name.Name) {
			pos := pass.Fset.Position(fd.Pos())
			if !shouldExcludeFile(pos.Filename, config) {
				pass.Reportf(fd.Pos(), "test function %s is not covered by goleak (%s)", fd.Name.Name, reason)
			}
		}
	})
}
