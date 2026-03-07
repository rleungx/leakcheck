package leakcheck

import (
	"context"
	"go/ast"
	"go/token"
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
	prepared := prepareConfig(config)

	return &analysis.Analyzer{
		Name: "leakcheck",
		Doc:  "check that all tests are covered by goleak",
		Run:  run(prepared),
	}
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
func run(config preparedConfig) func(*analysis.Pass) (interface{}, error) {
	return func(pass *analysis.Pass) (interface{}, error) {
		// Create context with timeout if specified
		ctx := context.Background()
		if config.timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, config.timeout)
			defer cancel()
		}

		// Use a channel to control concurrent processing
		semaphore := make(chan struct{}, config.concurrency)

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

		// Check if goleak is imported and get its alias
		goleakAlias := getGoleakAlias(pass.Files)

		// If no goleak import, report for all test functions
		if goleakAlias == "" {
			return reportUncoveredTestFunctionsWithContext(ctx, pass, config, "goleak not imported", semaphore)
		}

		// Check context again before expensive analysis
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Analyze test functions with context and worker control
		result, err := analyzeTestFunctionsWithContext(ctx, pass, goleakAlias, config, semaphore)
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
				reason := "missing defer goleak.VerifyNone(t)"
				if result.hasTestMain && !result.hasVerifyTestMain {
					reason = "TestMain exists but doesn't call goleak.VerifyTestMain"
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
	name     string
	pos      token.Pos
	filename string
	covered  bool
}

// analyzeTestFunctionsWithContext performs analysis with context and concurrency control
func analyzeTestFunctionsWithContext(ctx context.Context, pass *analysis.Pass, goleakAlias string, config preparedConfig, semaphore chan struct{}) (*analysisResult, error) {
	// For small number of files, use simple sequential processing
	if len(pass.Files) <= 3 {
		return analyzeTestFunctionsSequential(ctx, pass, goleakAlias, config)
	}

	result := &analysisResult{}

	var mu sync.Mutex // Protect shared result data

	// Process files with worker control
	var wg sync.WaitGroup
	errChan := make(chan error, 1)

	// Determine optimal worker count based on file count
	workerCount := cap(semaphore)
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
				localResult := processFileForAnalysis(file, pass, goleakAlias, config)

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
func analyzeTestFunctionsSequential(ctx context.Context, pass *analysis.Pass, goleakAlias string, config preparedConfig) (*analysisResult, error) {
	result := &analysisResult{}

	for _, file := range pass.Files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		localResult := processFileForAnalysis(file, pass, goleakAlias, config)
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
func processFileForAnalysis(file *ast.File, pass *analysis.Pass, goleakAlias string, config preparedConfig) *analysisResult {
	filename := pass.Fset.Position(file.Pos()).Filename
	if !isTestFile(filename) || shouldExcludeFileWithConfig(filename, config) {
		return emptyAnalysisResult()
	}

	result := &analysisResult{}
	visitor := newFileAnalysisVisitor(result, filename, goleakAlias)
	ast.Walk(&visitor.keep, file)
	return result
}

func emptyAnalysisResult() *analysisResult {
	return &analysisResult{}
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
		// Early exit if no imports
		if len(file.Imports) == 0 {
			continue
		}

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

	// Handle simple glob patterns (only convert if it looks like a simple glob)
	if strings.Contains(pattern, "*") && !containsRegexMetachars(pattern) {
		return matchGlobPattern(str, pattern)
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
func hasNonExcludedTestFiles(pass *analysis.Pass, config preparedConfig) bool {
	for _, file := range pass.Files {
		filename := pass.Fset.Position(file.Pos()).Filename
		if isTestFile(filename) && !shouldExcludeFileWithConfig(filename, config) {
			return true // Early return as soon as we find one
		}
	}
	return false
}

// reportUncoveredTestFunctionsWithContext reports all test functions that are not covered with context support.
func reportUncoveredTestFunctionsWithContext(ctx context.Context, pass *analysis.Pass, config preparedConfig, reason string, semaphore chan struct{}) (interface{}, error) {

	// Use semaphore to control concurrency
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case semaphore <- struct{}{}:
		defer func() { <-semaphore }()
	}

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
			if !ok || fd.Name == nil || fd.Recv != nil || !isTestFunction(fd.Name.Name) {
				continue
			}
			pass.Reportf(fd.Pos(), "test function %s is not covered by goleak (%s)", fd.Name.Name, reason)
		}
	}

	return nil, nil
}

type fileAnalysisVisitor struct {
	result      *analysisResult
	filename    string
	goleakAlias string
	stack       []*funcContext
	keep        visitorState
	pop         visitorState
}

type visitorState struct {
	owner *fileAnalysisVisitor
	pop   bool
}

type funcContext struct {
	isTestMain bool
	testFunc   *testFuncInfo
}

func newFileAnalysisVisitor(result *analysisResult, filename, goleakAlias string) *fileAnalysisVisitor {
	visitor := &fileAnalysisVisitor{
		result:      result,
		filename:    filename,
		goleakAlias: goleakAlias,
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
		case isTestFunction(funcName):
			v.result.testFuncs = append(v.result.testFuncs, testFuncInfo{
				name:     funcName,
				pos:      node.Pos(),
				filename: v.filename,
			})
			v.stack = append(v.stack, &funcContext{testFunc: &v.result.testFuncs[len(v.result.testFuncs)-1]})
			return &v.pop
		default:
			return nil
		}
	case *ast.CallExpr:
		ctx := v.current()
		if ctx != nil && ctx.isTestMain {
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok && isGoleakCall(sel, verifyTestMain, v.goleakAlias) {
				v.result.hasVerifyTestMain = true
			}
		}
	case *ast.DeferStmt:
		ctx := v.current()
		if ctx != nil && ctx.testFunc != nil {
			if call, ok := node.Call.Fun.(*ast.SelectorExpr); ok && isGoleakCall(call, verifyNone, v.goleakAlias) {
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
