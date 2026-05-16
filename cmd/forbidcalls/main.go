package main

import (
	"github.com/t3spe/forbidcalls"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(forbidcalls.StandaloneAnalyzer())
}
