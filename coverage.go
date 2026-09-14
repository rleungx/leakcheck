package leakcheck

import (
	"go/ast"
	"go/constant"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/cfg"
)

// Track deferred and already-executed checks separately: os.Exit skips defers.
const (
	deferredCheck uint8 = 1 << iota
	executedCheck
)

// hasGoleakCoverage requires a check on every reachable exit path. It is an
// intraprocedural check: closures, goroutines, and calls through helpers do not
// establish coverage. Unknown calls are assumed to return normally.
func hasGoleakCoverage(pass *analysis.Pass, fd *ast.FuncDecl, imports goleakImports, main bool) bool {
	graph := cfg.New(fd.Body, func(call *ast.CallExpr) bool {
		return callTermination(pass, call) == returnsNormally
	})
	type state struct {
		block  *cfg.Block
		checks uint8
	}
	work := []state{{block: graph.Blocks[0]}}
	visited := make([][4]bool, len(graph.Blocks))
	foundCheck := false
	method := verifyNone
	if main {
		method = verifyTestMain
	}
paths:
	for len(work) > 0 {
		current := work[len(work)-1]
		work = work[:len(work)-1]
		if visited[current.block.Index][current.checks] {
			continue
		}
		visited[current.block.Index][current.checks] = true
		checks := current.checks
		for _, node := range current.block.Nodes {
			switch stmt := node.(type) {
			case *ast.DeferStmt:
				if isGoleakCall(pass, stmt.Call.Fun, method, imports) {
					checks |= deferredCheck
					foundCheck = true
				}
			case *ast.ExprStmt:
				call, ok := ast.Unparen(stmt.X).(*ast.CallExpr)
				if !ok {
					continue
				}
				if main && isGoleakCall(pass, call.Fun, method, imports) {
					checks |= executedCheck
					foundCheck = true
				}
				switch callTermination(pass, call) {
				case exitsWithDefers:
					if checks == 0 {
						return false
					}
					continue paths
				case exitsProcess:
					if checks&executedCheck == 0 {
						return false
					}
					continue paths
				}
			case *ast.ReturnStmt:
				if checks == 0 {
					return false
				}
			}
		}
		for _, successor := range reachableSuccessors(pass, current.block) {
			work = append(work, state{block: successor, checks: checks})
		}
	}
	return foundCheck
}

// cfg deliberately retains both constant-condition branches. Prune only if/for
// edges here; switch edges represent comparisons, not the case expression alone.
func reachableSuccessors(pass *analysis.Pass, block *cfg.Block) []*cfg.Block {
	if pass.TypesInfo == nil || len(block.Succs) != 2 || len(block.Nodes) == 0 {
		return block.Succs
	}
	var condition ast.Expr
	switch stmt := block.Succs[0].Stmt.(type) {
	case *ast.IfStmt:
		condition = stmt.Cond
	case *ast.ForStmt:
		condition = stmt.Cond
	}
	if condition == nil || block.Nodes[len(block.Nodes)-1] != condition {
		return block.Succs
	}
	value := pass.TypesInfo.Types[condition].Value
	if value == nil || value.Kind() != constant.Bool {
		return block.Succs
	}
	if constant.BoolVal(value) {
		return block.Succs[:1]
	}
	return block.Succs[1:]
}

type termination uint8

const (
	returnsNormally termination = iota
	exitsWithDefers
	exitsProcess
)

// Recognize standard no-return calls by their resolved objects, not by alias
// spelling. This also handles promoted testing.T methods and log.Logger.Fatal.
func callTermination(pass *analysis.Pass, call *ast.CallExpr) termination {
	if pass.TypesInfo == nil {
		return returnsNormally
	}
	var ident *ast.Ident
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		ident = fun
	case *ast.SelectorExpr:
		ident = fun.Sel
	default:
		return returnsNormally
	}
	obj := pass.TypesInfo.Uses[ident]
	if builtin, ok := obj.(*types.Builtin); ok && builtin.Name() == "panic" {
		return exitsWithDefers
	}
	fn, ok := obj.(*types.Func)
	if !ok || fn.Pkg() == nil {
		return returnsNormally
	}
	switch fn.Pkg().Path() {
	case "os":
		if fn.Name() == "Exit" {
			return exitsProcess
		}
	case "log":
		switch fn.Name() {
		case "Fatal", "Fatalf", "Fatalln":
			return exitsProcess
		case "Panic", "Panicf", "Panicln":
			return exitsWithDefers
		}
	case "runtime":
		if fn.Name() == "Goexit" {
			return exitsWithDefers
		}
	case "testing":
		switch fn.Name() {
		case "FailNow", "Fatal", "Fatalf", "Skip", "Skipf", "SkipNow":
			return exitsWithDefers
		}
	}
	return returnsNormally
}
