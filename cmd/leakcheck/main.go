package main

import (
	"github.com/rleungx/leakcheck"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(leakcheck.Analyzer)
}
