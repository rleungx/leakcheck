package control_flow

import (
	"log"
	"os"
	"runtime"
	"testing"

	"go.uber.org/goleak"
)

var condition bool

func TestDeadBranch(t *testing.T) { // want "test function TestDeadBranch is not covered by goleak"
	if false {
		defer goleak.VerifyNone(t)
	}
}

func TestConditional(t *testing.T) { // want "test function TestConditional is not covered by goleak"
	if condition {
		defer goleak.VerifyNone(t)
	}
}

func TestEarlyReturn(t *testing.T) { // want "test function TestEarlyReturn is not covered by goleak"
	return
	defer goleak.VerifyNone(t)
}

func TestConditionalReturn(t *testing.T) { // want "test function TestConditionalReturn is not covered by goleak"
	if condition {
		return
	}
	defer goleak.VerifyNone(t)
}

func TestZeroIterations(t *testing.T) { // want "test function TestZeroIterations is not covered by goleak"
	for condition {
		defer goleak.VerifyNone(t)
		break
	}
}

func TestDeadLoop(t *testing.T) { // want "test function TestDeadLoop is not covered by goleak"
	for false {
		defer goleak.VerifyNone(t)
	}
}

func TestGotoBypass(t *testing.T) { // want "test function TestGotoBypass is not covered by goleak"
	if condition {
		goto done
	}
	defer goleak.VerifyNone(t)
done:
	return
}

func TestSwitchMissingDefault(t *testing.T) { // want "test function TestSwitchMissingDefault is not covered by goleak"
	switch condition {
	case true:
		defer goleak.VerifyNone(t)
	}
}

func TestSelectMissingBranch(t *testing.T) { // want "test function TestSelectMissingBranch is not covered by goleak"
	select {
	case <-make(chan struct{}):
		defer goleak.VerifyNone(t)
	default:
	}
}

func TestDirectCall(t *testing.T) { // want "test function TestDirectCall is not covered by goleak"
	goleak.VerifyNone(t)
}

func TestPanicBeforeDefer(t *testing.T) { // want "test function TestPanicBeforeDefer is not covered by goleak"
	panic("stop")
	defer goleak.VerifyNone(t)
}

func TestFatalBeforeDefer(t *testing.T) { // want "test function TestFatalBeforeDefer is not covered by goleak"
	t.Fatal("stop")
	defer goleak.VerifyNone(t)
}

func TestSkipBeforeDefer(t *testing.T) { // want "test function TestSkipBeforeDefer is not covered by goleak"
	t.Skip("stop")
	defer goleak.VerifyNone(t)
}

func TestGoexitBeforeDefer(t *testing.T) { // want "test function TestGoexitBeforeDefer is not covered by goleak"
	runtime.Goexit()
	defer goleak.VerifyNone(t)
}

func TestExitSkipsDefer(t *testing.T) { // want "test function TestExitSkipsDefer is not covered by goleak"
	defer goleak.VerifyNone(t)
	os.Exit(0)
}

func TestLogFatalSkipsDefer(t *testing.T) { // want "test function TestLogFatalSkipsDefer is not covered by goleak"
	defer goleak.VerifyNone(t)
	log.Fatal("stop")
}

func TestBothBranches(t *testing.T) {
	if condition {
		defer goleak.VerifyNone(t)
	} else {
		defer goleak.VerifyNone(t)
	}
}

func TestConstantTrue(t *testing.T) {
	if true {
		defer goleak.VerifyNone(t)
	}
}

func TestConstantElse(t *testing.T) {
	if false {
		return
	} else {
		defer goleak.VerifyNone(t)
	}
}

func TestLoopWithCheck(t *testing.T) {
	for {
		defer goleak.VerifyNone(t)
		break
	}
}

func TestConstantLoop(t *testing.T) {
	for true {
		defer goleak.VerifyNone(t)
		break
	}
}

func TestAfterJoin(t *testing.T) {
	if condition {
		t.Log("condition")
	}
	defer goleak.VerifyNone(t)
}

func TestSwitchCovered(t *testing.T) {
	switch condition {
	case false:
		defer goleak.VerifyNone(t)
	default:
		defer goleak.VerifyNone(t)
	}
}

func TestSelectCovered(t *testing.T) {
	select {
	case <-make(chan struct{}):
		defer goleak.VerifyNone(t)
	default:
		defer goleak.VerifyNone(t)
	}
}

func TestPanicWithDefer(t *testing.T) {
	defer goleak.VerifyNone(t)
	panic("stop")
}

func TestFatalWithDefer(t *testing.T) {
	defer goleak.VerifyNone(t)
	t.Fatal("stop")
}

func TestParenthesizedPanic(t *testing.T) {
	defer goleak.VerifyNone(t)
	(panic("stop"))
	os.Exit(0) // Unreachable even though cfg does not split parenthesized calls.
}

func TestShadowedExit(t *testing.T) {
	os := struct{ Exit func(int) }{Exit: func(int) {}}
	os.Exit(0)
	defer (goleak.VerifyNone)(t)
}
