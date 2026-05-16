package forbidcalls

import (
	"flag"
	"fmt"
	"sync"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

// StandaloneAnalyzer returns an analyzer that loads its Config lazily from
// the file path passed via the `-config` flag. Designed for use with
// singlechecker / multichecker. Config loading is deferred to the first
// Run call so that flag parsing has completed.
func StandaloneAnalyzer() *analysis.Analyzer {
	var fs flag.FlagSet
	configPath := fs.String("config", ".forbidcalls.yaml",
		"path to forbidcalls YAML config")

	var (
		once    sync.Once
		runFn   func(*analysis.Pass) (any, error)
		initErr error
	)

	a := &analysis.Analyzer{
		Name:     "forbidcalls",
		Doc:      "flags references to forbidden functions, methods, or whole-package wildcards.",
		Flags:    fs,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}
	a.Run = func(pass *analysis.Pass) (any, error) {
		once.Do(func() {
			cfg, err := LoadConfig(*configPath)
			if err != nil {
				initErr = fmt.Errorf("load config %q: %w", *configPath, err)
				return
			}
			inner, err := NewAnalyzer(*cfg)
			if err != nil {
				initErr = err
				return
			}
			runFn = inner.Run
		})
		if initErr != nil {
			return nil, initErr
		}
		return runFn(pass)
	}
	return a
}
