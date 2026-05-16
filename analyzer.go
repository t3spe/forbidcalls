package forbidcalls

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const ignoreDirective = "//forbidcalls:ignore"

// NewAnalyzer builds an analyzer that reports references to APIs matching
// any of the configured Forbid patterns. References include calls, value
// captures (`f := os.Getenv`), and method values.
func NewAnalyzer(cfg Config) (*analysis.Analyzer, error) {
	patterns, err := ParsePatterns(cfg.Forbid)
	if err != nil {
		return nil, err
	}
	return &analysis.Analyzer{
		Name:     "forbidcalls",
		Doc:      "flags references to forbidden functions, methods, or whole-package wildcards.",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Run: func(pass *analysis.Pass) (any, error) {
			return run(pass, patterns, cfg)
		},
	}, nil
}

func run(pass *analysis.Pass, patterns []Pattern, cfg Config) (any, error) {
	insp, _ := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if insp == nil {
		return nil, fmt.Errorf("inspect analyzer result missing")
	}

	ignored := collectIgnoredLines(pass)

	insp.Preorder([]ast.Node{(*ast.SelectorExpr)(nil)}, func(n ast.Node) {
		sel := n.(*ast.SelectorExpr)
		obj := pass.TypesInfo.Uses[sel.Sel]
		if obj == nil || obj.Pkg() == nil {
			return
		}

		pos := pass.Fset.Position(sel.Pos())
		if cfg.FileExcluded(pos.Filename) {
			return
		}
		if ignored[pos.Filename][pos.Line] {
			return
		}

		for _, p := range patterns {
			if matches(obj, p) {
				pass.Reportf(sel.Pos(),
					"forbidden reference to %s (pattern: %s)",
					qualifiedName(obj), p.Raw)
				return
			}
		}
	})
	return nil, nil
}

// matches reports whether obj satisfies pattern p.
func matches(obj types.Object, p Pattern) bool {
	if obj.Pkg() == nil || obj.Pkg().Path() != p.PkgPath {
		return false
	}

	if p.Receiver == "" {
		// Plain function / var pattern.
		if p.Name == "*" {
			return obj.Exported()
		}
		return obj.Name() == p.Name
	}

	// Method pattern: obj must be a method with matching receiver.
	fn, ok := obj.(*types.Func)
	if !ok {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return false
	}

	recvStr, ok := receiverString(sig.Recv().Type())
	if !ok || recvStr != p.Receiver {
		return false
	}

	if p.Name == "*" {
		return fn.Exported()
	}
	return fn.Name() == p.Name
}

// receiverString normalizes a receiver type to "Type" or "*Type".
func receiverString(t types.Type) (string, bool) {
	if ptr, ok := t.(*types.Pointer); ok {
		named, ok := ptr.Elem().(*types.Named)
		if !ok {
			return "", false
		}
		return "*" + named.Obj().Name(), true
	}
	if named, ok := t.(*types.Named); ok {
		return named.Obj().Name(), true
	}
	return "", false
}

func qualifiedName(obj types.Object) string {
	if obj.Pkg() == nil {
		return obj.Name()
	}
	return obj.Pkg().Path() + "." + obj.Name()
}

// collectIgnoredLines builds a per-file set of line numbers where a
// //forbidcalls:ignore directive suppresses diagnostics. A directive
// associates via ast.CommentMap with the nearest AST node, so both inline
// trailing comments and standalone leading comments work.
func collectIgnoredLines(pass *analysis.Pass) map[string]map[int]bool {
	ignored := make(map[string]map[int]bool)
	for _, f := range pass.Files {
		cmap := ast.NewCommentMap(pass.Fset, f, f.Comments)
		for node, groups := range cmap {
			for _, g := range groups {
				for _, c := range g.List {
					if !strings.HasPrefix(c.Text, ignoreDirective) {
						continue
					}
					pos := pass.Fset.Position(node.Pos())
					if ignored[pos.Filename] == nil {
						ignored[pos.Filename] = make(map[int]bool)
					}
					// Mark every line spanned by the associated node.
					end := pass.Fset.Position(node.End())
					for line := pos.Line; line <= end.Line; line++ {
						ignored[pos.Filename][line] = true
					}
				}
			}
		}
	}
	return ignored
}
