package leakcheck

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the goleak coverage linter analyzer
var Analyzer = &analysis.Analyzer{
	Name:     "leakcheck",
	Doc:      "check that all tests are covered by goleak",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// Check if we have any files to analyze
	if len(pass.Files) == 0 {
		return nil, nil
	}

	// Check all files for test files
	hasTestFile := false
	for _, file := range pass.Files {
		filename := pass.Fset.Position(file.Pos()).Filename
		if isTestFile(filename) {
			hasTestFile = true
			break
		}
	}

	if !hasTestFile {
		return nil, nil
	}

	var (
		hasTestMain         bool
		hasVerifyTestMain   bool
		testFuncs           []string
		funcsCoveredByDefer map[string]bool = make(map[string]bool)
	)

	// Look for imports to check if goleak is imported
	hasGoleakImport := false
	for _, file := range pass.Files {
		for _, imp := range file.Imports {
			if imp.Path != nil && (imp.Path.Value == `"go.uber.org/goleak"` || imp.Path.Value == `"github.com/uber-go/goleak"`) {
				hasGoleakImport = true
				break
			}
		}
	}

	// If no goleak import, report for all test functions
	if !hasGoleakImport {
		inspect.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node) {
			fd := n.(*ast.FuncDecl)
			if isTestFunction(fd.Name.Name) {
				pass.Reportf(fd.Pos(), "test function %s is not covered by goleak (goleak not imported)", fd.Name.Name)
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
						if isGoleakCall(sel, "VerifyTestMain") {
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
						if isGoleakCall(call, "VerifyNone") {
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
					if hasTestMain && !hasVerifyTestMain {
						pass.Reportf(fd.Pos(), "test function %s is not covered by goleak (TestMain exists but doesn't call goleak.VerifyTestMain)", testFunc)
					} else {
						pass.Reportf(fd.Pos(), "test function %s is not covered by goleak (missing defer goleak.VerifyNone(t))", testFunc)
					}
				}
			})
		}
	}

	return nil, nil
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
func isGoleakCall(sel *ast.SelectorExpr, method string) bool {
	if sel.Sel.Name != method {
		return false
	}

	if ident, ok := sel.X.(*ast.Ident); ok {
		return ident.Name == "goleak"
	}

	return false
}
