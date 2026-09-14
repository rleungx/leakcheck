package leakcheck

import (
	"context"
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/tools/go/analysis"
)

// Config holds the configuration for the leakcheck analyzer
type Config struct {
	ExcludePackages string
	ExcludeFiles    string
	Concurrency     int
	Timeout         time.Duration
}

type preparedConfig struct {
	excludePackagePatterns []string
	excludeFilePatterns    []string
	concurrency            int
	timeout                time.Duration
}

// regexCache caches compiled regular expressions for better performance
var (
	regexCache = make(map[string]*regexp.Regexp, 16) // Pre-allocate with reasonable capacity
	regexMutex sync.RWMutex
)

// New creates a new leakcheck analyzer with default configuration
func New() *analysis.Analyzer {
	return NewWithConfig(&Config{})
}

// NewWithConfig creates a new leakcheck analyzer with custom configuration
func NewWithConfig(config *Config) *analysis.Analyzer {
	effectiveConfig := &Config{}
	if config != nil {
		*effectiveConfig = *config
	}

	analyzer := &analysis.Analyzer{
		Name: "leakcheck",
		Doc:  "check that all tests are covered by goleak",
		Run:  run(effectiveConfig),
	}
	analyzer.Flags.StringVar(&effectiveConfig.ExcludePackages, "exclude-packages", effectiveConfig.ExcludePackages, "comma-separated list of package patterns to exclude (supports regex and glob patterns)")
	analyzer.Flags.StringVar(&effectiveConfig.ExcludeFiles, "exclude-files", effectiveConfig.ExcludeFiles, "comma-separated list of file patterns to exclude (supports regex and glob patterns)")
	analyzer.Flags.IntVar(&effectiveConfig.Concurrency, "concurrency", effectiveConfig.Concurrency, "number of concurrent file workers (default: number of CPUs)")
	analyzer.Flags.DurationVar(&effectiveConfig.Timeout, "timeout", effectiveConfig.Timeout, "per-package analysis timeout (default: 30m)")
	return analyzer
}

// Analyzer is the default analyzer instance for backward compatibility
var Analyzer = New()

func prepareConfig(config *Config) preparedConfig {
	if config == nil {
		config = &Config{}
	}

	prepared := preparedConfig{
		excludePackagePatterns: splitPatterns(config.ExcludePackages),
		excludeFilePatterns:    splitPatterns(config.ExcludeFiles),
		concurrency:            config.Concurrency,
		timeout:                config.Timeout,
	}

	if prepared.concurrency <= 0 {
		prepared.concurrency = runtime.NumCPU()
	}
	if prepared.timeout <= 0 {
		prepared.timeout = 30 * time.Minute
	}

	return prepared
}

// run creates a run function with the given configuration
func run(rawConfig *Config) func(*analysis.Pass) (interface{}, error) {
	return func(pass *analysis.Pass) (interface{}, error) {
		config := prepareConfig(rawConfig)

		// Create context with timeout if specified
		ctx := context.Background()
		if config.timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, config.timeout)
			defer cancel()
		}

		// Early bailout checks for performance
		if len(pass.Files) == 0 {
			return nil, nil
		}

		// Check context for timeout
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Check if package should be excluded first (fastest check)
		if shouldExcludePackage(pass.Pkg.Path(), config) {
			return nil, nil
		}

		// Check if we have any non-excluded test files
		if !hasNonExcludedTestFiles(pass, config) {
			return nil, nil
		}

		// Preserve the common no-import fast path while keeping import aliases
		// file-scoped when at least one test file imports goleak.
		if !hasAnyGoleakImport(pass.Files) {
			return reportUncoveredTestFunctions(ctx, pass, config)
		}

		// Analyze test functions with context and worker control
		result, err := analyzeTestFunctionsWithContext(ctx, pass, config)
		if err != nil {
			return nil, err
		}

		// Report issues
		if result.hasTestMain && result.hasVerifyTestMain {
			// If TestMain with VerifyTestMain exists, all tests are covered
			return nil, nil
		}

		// Check individual test functions with context
		for _, testFunc := range result.testFuncs {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			if !testFunc.covered {
				reason := "goleak not imported"
				if result.hasTestMain {
					reason = "TestMain exists but doesn't call goleak.VerifyTestMain"
				} else if testFunc.hasGoleakImport {
					reason = "missing defer goleak.VerifyNone(t)"
				}
				// Report directly using cached position info
				if !shouldExcludeFileWithConfig(testFunc.filename, config) {
					pass.Reportf(testFunc.pos, "test function %s is not covered by goleak (%s)", testFunc.name, reason)
				}
			}
		}

		return nil, nil
	}
}

// analysisResult holds the analysis results to avoid multiple traversals
type analysisResult struct {
	hasTestMain       bool
	hasVerifyTestMain bool
	testFuncs         []testFuncInfo
}

// testFuncInfo holds information about a test function
type testFuncInfo struct {
	name            string
	pos             token.Pos
	filename        string
	hasGoleakImport bool
	covered         bool
}

// analyzeTestFunctionsWithContext performs analysis with context and concurrency control
func analyzeTestFunctionsWithContext(ctx context.Context, pass *analysis.Pass, config preparedConfig) (*analysisResult, error) {
	// For small number of files, use simple sequential processing
	if len(pass.Files) <= 3 {
		return analyzeTestFunctionsSequential(ctx, pass, config)
	}

	result := &analysisResult{}

	var mu sync.Mutex // Protect shared result data

	// Process files with worker control
	var wg sync.WaitGroup
	errChan := make(chan error, 1)

	// Determine optimal worker count based on file count
	workerCount := config.concurrency
	if len(pass.Files) < workerCount {
		workerCount = len(pass.Files)
	}

	// Create a channel to control file processing
	fileChan := make(chan *ast.File, len(pass.Files))
	for _, file := range pass.Files {
		fileChan <- file
	}
	close(fileChan)

	// Start workers to process files
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for file := range fileChan {
				select {
				case <-ctx.Done():
					select {
					case errChan <- ctx.Err():
					default:
					}
					return
				default:
				}

				// Process this file
				localResult := processFileForAnalysis(file, pass, config)

				// Merge results with mutex protection
				mu.Lock()
				mergeResults(result, localResult)
				mu.Unlock()
			}
		}()
	}

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Check for errors or completion
	select {
	case err := <-errChan:
		if err != nil {
			return nil, err
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return result, nil
}

// analyzeTestFunctionsSequential performs sequential analysis for small number of files
func analyzeTestFunctionsSequential(ctx context.Context, pass *analysis.Pass, config preparedConfig) (*analysisResult, error) {
	result := &analysisResult{}

	for _, file := range pass.Files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		localResult := processFileForAnalysis(file, pass, config)
		mergeResults(result, localResult)
	}

	return result, nil
}

// mergeResults efficiently merges local result into the main result
func mergeResults(result, localResult *analysisResult) {
	if localResult.hasTestMain {
		result.hasTestMain = true
	}
	if localResult.hasVerifyTestMain {
		result.hasVerifyTestMain = true
	}
	result.testFuncs = append(result.testFuncs, localResult.testFuncs...)
}

// processFileForAnalysis processes a single file for test function analysis.
func processFileForAnalysis(file *ast.File, pass *analysis.Pass, config preparedConfig) *analysisResult {
	filename := pass.Fset.Position(file.Pos()).Filename
	if !isTestFile(filename) {
		return emptyAnalysisResult()
	}

	result := &analysisResult{}
	imports := getGoleakImports(file)
	includeTests := !shouldExcludeFileWithConfig(filename, config)
	if !imports.imported {
		return analyzeFileWithoutGoleakImport(file, filename, includeTests)
	}

	visitor := newFileAnalysisVisitor(result, pass, filename, imports, includeTests)
	ast.Walk(&visitor.keep, file)
	return result
}

func analyzeFileWithoutGoleakImport(file *ast.File, filename string, includeTests bool) *analysisResult {
	result := &analysisResult{}
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Name == nil || fd.Body == nil || fd.Recv != nil {
			continue
		}
		switch {
		case fd.Name.Name == testMainFunc:
			result.hasTestMain = true
		case includeTests && isTestFunction(fd.Name.Name):
			result.testFuncs = append(result.testFuncs, testFuncInfo{
				name:     fd.Name.Name,
				pos:      fd.Pos(),
				filename: filename,
			})
		}
	}
	return result
}

func emptyAnalysisResult() *analysisResult {
	return &analysisResult{}
}

// Constants for goleak package paths and method names
const (
	goleakUberPath   = "go.uber.org/goleak"
	goleakGithubPath = "github.com/uber-go/goleak"
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
	if name == testMainFunc {
		return false
	}
	return isGoTestName(name, testPrefix)
}

func isGoTestName(name, prefix string) bool {
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	if len(name) == len(prefix) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(name[len(prefix):])
	return !unicode.IsLower(r)
}

type goleakImports struct {
	names    map[string]struct{}
	dot      bool
	imported bool
}

func getGoleakImports(file *ast.File) goleakImports {
	imports := goleakImports{}
	for _, imp := range file.Imports {
		if imp.Path == nil || !isGoleakPath(strings.Trim(imp.Path.Value, `"`)) {
			continue
		}

		imports.imported = true
		name := defaultAlias
		if imp.Name != nil {
			name = imp.Name.Name
		}
		switch name {
		case ".":
			imports.dot = true
		case "_":
		default:
			if imports.names == nil {
				imports.names = make(map[string]struct{}, 1)
			}
			imports.names[name] = struct{}{}
		}
	}
	return imports
}

func isGoleakPath(path string) bool {
	return isImportedPath(path, goleakUberPath) || isImportedPath(path, goleakGithubPath)
}

func isImportedPath(actual, imported string) bool {
	return actual == imported || actual == "vendor/"+imported || strings.HasSuffix(actual, "/vendor/"+imported)
}

// isGoleakCall checks the resolved function object rather than trusting the
// spelling of a package alias, which may be different in each file or shadowed.
func isGoleakCall(pass *analysis.Pass, fun ast.Expr, method string, imports goleakImports) bool {
	var ident *ast.Ident
	switch expr := fun.(type) {
	case *ast.SelectorExpr:
		ident = expr.Sel
	case *ast.Ident:
		ident = expr
	default:
		return false
	}

	if ident.Name != method {
		return false
	}
	if pass.TypesInfo != nil {
		fn, ok := pass.TypesInfo.Uses[ident].(*types.Func)
		return ok && fn.Pkg() != nil && isGoleakPath(fn.Pkg().Path())
	}

	// A small fallback keeps direct Analyzer.Run users and benchmarks working
	// even if they construct a Pass without type information.
	switch expr := fun.(type) {
	case *ast.SelectorExpr:
		pkg, ok := expr.X.(*ast.Ident)
		if !ok {
			return false
		}
		_, ok = imports.names[pkg.Name]
		return ok
	case *ast.Ident:
		return imports.dot
	default:
		return false
	}
}

// shouldExcludePackage checks if a package should be excluded
func shouldExcludePackage(pkgPath string, config preparedConfig) bool {
	return matchesAnyPattern(pkgPath, config.excludePackagePatterns)
}

// shouldExcludeFileWithConfig checks if a file should be excluded
func shouldExcludeFileWithConfig(filename string, config preparedConfig) bool {
	// Extract just the filename without path for pattern matching
	justFilename := filename
	if lastSlash := strings.LastIndex(filename, "/"); lastSlash >= 0 {
		justFilename = filename[lastSlash+1:]
	}
	if lastBackslash := strings.LastIndex(justFilename, "\\"); lastBackslash >= 0 {
		justFilename = justFilename[lastBackslash+1:]
	}

	return matchesAnyPattern(filename, config.excludeFilePatterns) || matchesAnyPattern(justFilename, config.excludeFilePatterns)
}

func splitPatterns(patterns string) []string {
	if patterns == "" {
		return nil
	}

	if !strings.Contains(patterns, ",") {
		pattern := strings.TrimSpace(patterns)
		if pattern == "" {
			return nil
		}
		return []string{pattern}
	}

	parts := strings.Split(patterns, ",")
	result := parts[:0]
	for _, pattern := range parts {
		pattern = strings.TrimSpace(pattern)
		if pattern != "" {
			result = append(result, pattern)
		}
	}
	return result
}

// matchesAnyPattern checks if a string matches any prepared pattern.
func matchesAnyPattern(str string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}

	if len(patterns) == 1 {
		return matchesPattern(str, patterns[0])
	}

	for _, pattern := range patterns {
		if matchesPattern(str, pattern) {
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

	// Fast path: empty pattern means no match
	if pattern == "" {
		return false
	}

	// Fast path: pattern as substring (common case for package exclusions)
	if !containsSpecialChars(pattern) {
		return strings.Contains(str, pattern)
	}

	// Try glob semantics first. If the pattern was intended as a regular
	// expression, a failed glob match falls through to the regex matcher.
	if strings.Contains(pattern, "*") && matchGlobPattern(str, pattern) {
		return true
	}

	// Try regex match with caching for complex patterns
	return matchRegexPattern(str, pattern)
}

// matchGlobPattern handles simple glob patterns efficiently
func matchGlobPattern(str, pattern string) bool {
	// Convert glob to regex
	regexPattern := strings.ReplaceAll(regexp.QuoteMeta(pattern), `\*`, ".*")
	regexPattern = "^" + regexPattern + "$"

	// Use regex cache for compiled glob patterns
	regexMutex.RLock()
	re, ok := regexCache[regexPattern]
	regexMutex.RUnlock()

	if !ok {
		var err error
		re, err = regexp.Compile(regexPattern)
		if err != nil {
			return false
		}

		regexMutex.Lock()
		// Check cache size and clean if necessary
		if len(regexCache) > 100 {
			// Keep only recent entries - simple LRU-like behavior
			for k := range regexCache {
				delete(regexCache, k)
				if len(regexCache) <= 50 {
					break
				}
			}
		}
		regexCache[regexPattern] = re
		regexMutex.Unlock()
	}

	return re.MatchString(str)
}

// matchRegexPattern handles regex patterns with caching
func matchRegexPattern(str, pattern string) bool {
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
		// Check cache size and clean if necessary
		if len(regexCache) > 100 {
			// Keep only recent entries - simple LRU-like behavior
			for k := range regexCache {
				delete(regexCache, k)
				if len(regexCache) <= 50 {
					break
				}
			}
		}
		regexCache[pattern] = re
		regexMutex.Unlock()
	}

	return re.MatchString(str)
}

// containsSpecialChars checks if pattern contains special characters that need regex handling
func containsSpecialChars(pattern string) bool {
	return strings.ContainsAny(pattern, ".*+?^${}()[]|\\")
}

// hasNonExcludedTestFiles checks if there are any test files that are not excluded
func hasNonExcludedTestFiles(pass *analysis.Pass, config preparedConfig) bool {
	for _, file := range pass.Files {
		filename := pass.Fset.Position(file.Pos()).Filename
		if isTestFile(filename) && !shouldExcludeFileWithConfig(filename, config) {
			return true // Early return as soon as we find one
		}
	}
	return false
}

func hasAnyGoleakImport(files []*ast.File) bool {
	for _, file := range files {
		if getGoleakImports(file).imported {
			return true
		}
	}
	return false
}

func reportUncoveredTestFunctions(ctx context.Context, pass *analysis.Pass, config preparedConfig) (interface{}, error) {
	for _, file := range pass.Files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		filename := pass.Fset.Position(file.Pos()).Filename
		if !isTestFile(filename) || shouldExcludeFileWithConfig(filename, config) {
			continue
		}
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Name == nil || fd.Body == nil || fd.Recv != nil || !isTestFunction(fd.Name.Name) {
				continue
			}
			pass.Reportf(fd.Pos(), "test function %s is not covered by goleak (goleak not imported)", fd.Name.Name)
		}
	}
	return nil, nil
}

type fileAnalysisVisitor struct {
	result       *analysisResult
	pass         *analysis.Pass
	filename     string
	imports      goleakImports
	includeTests bool
	stack        []*funcContext
	keep         visitorState
	pop          visitorState
}

type visitorState struct {
	owner *fileAnalysisVisitor
	pop   bool
}

type funcContext struct {
	isTestMain bool
	testFunc   *testFuncInfo
}

func newFileAnalysisVisitor(result *analysisResult, pass *analysis.Pass, filename string, imports goleakImports, includeTests bool) *fileAnalysisVisitor {
	visitor := &fileAnalysisVisitor{
		result:       result,
		pass:         pass,
		filename:     filename,
		imports:      imports,
		includeTests: includeTests,
	}
	visitor.keep.owner = visitor
	visitor.pop.owner = visitor
	visitor.pop.pop = true
	return visitor
}

func (s *visitorState) Visit(n ast.Node) ast.Visitor {
	return s.owner.visit(n, s.pop)
}

func (v *fileAnalysisVisitor) visit(n ast.Node, shouldPop bool) ast.Visitor {
	if n == nil {
		if shouldPop && len(v.stack) > 0 {
			v.stack = v.stack[:len(v.stack)-1]
		}
		return nil
	}

	switch node := n.(type) {
	case *ast.FuncLit:
		return nil
	case *ast.FuncDecl:
		if node.Name == nil || node.Body == nil || node.Recv != nil {
			return nil
		}

		funcName := node.Name.Name
		switch {
		case funcName == testMainFunc:
			v.result.hasTestMain = true
			v.stack = append(v.stack, &funcContext{isTestMain: true})
			return &v.pop
		case v.includeTests && isTestFunction(funcName):
			v.result.testFuncs = append(v.result.testFuncs, testFuncInfo{
				name:            funcName,
				pos:             node.Pos(),
				filename:        v.filename,
				hasGoleakImport: v.imports.imported,
			})
			v.stack = append(v.stack, &funcContext{testFunc: &v.result.testFuncs[len(v.result.testFuncs)-1]})
			return &v.pop
		default:
			return nil
		}
	case *ast.CallExpr:
		ctx := v.current()
		if ctx != nil && ctx.isTestMain {
			if isGoleakCall(v.pass, node.Fun, verifyTestMain, v.imports) {
				v.result.hasVerifyTestMain = true
			}
		}
	case *ast.DeferStmt:
		ctx := v.current()
		if ctx != nil && ctx.testFunc != nil {
			if isGoleakCall(v.pass, node.Call.Fun, verifyNone, v.imports) {
				ctx.testFunc.covered = true
			}
		}
	}

	return &v.keep
}

func (v *fileAnalysisVisitor) current() *funcContext {
	if len(v.stack) == 0 {
		return nil
	}
	return v.stack[len(v.stack)-1]
}
