package forbidcalls

import (
	"fmt"
	"go/ast"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const ignoreDirective = "//forbidcalls:ignore"

// NewAnalyzer builds an analyzer that reports references to APIs matching
// any of the configured Forbid patterns. References include calls, value
// captures (`f := os.Getenv`), and method values.
//
// Multiple rules are evaluated first-match-wins: once a reference matches
// some rule (with the file not in that rule's exclude_files), a single
// diagnostic fires and later rules are skipped for that reference.
func NewAnalyzer(cfg Config) (*analysis.Analyzer, error) {
	if err := cfg.Normalize(); err != nil {
		return nil, err
	}
	rules, err := compileRules(cfg.Rules)
	if err != nil {
		return nil, err
	}
	return &analysis.Analyzer{
		Name:     "forbidcalls",
		Doc:      "flags references to forbidden functions, methods, or whole-package wildcards.",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Run: func(pass *analysis.Pass) (any, error) {
			return run(pass, rules)
		},
	}, nil
}

// compiledRule pairs a rule's parsed patterns with its exclude list and
// optional name. Built once at analyzer construction; reused across
// every analysis pass.
type compiledRule struct {
	name     string
	patterns []Pattern
	excludes []string
}

func compileRules(rules []Rule) ([]compiledRule, error) {
	out := make([]compiledRule, 0, len(rules))
	for i, r := range rules {
		ps, err := ParsePatterns(r.Forbid)
		if err != nil {
			if r.Name != "" {
				return nil, fmt.Errorf("rule %d (%q): %w", i, r.Name, err)
			}
			return nil, fmt.Errorf("rule %d: %w", i, err)
		}
		out = append(out, compiledRule{
			name:     r.Name,
			patterns: ps,
			excludes: r.ExcludeFiles,
		})
	}
	return out, nil
}

func (r compiledRule) fileExcluded(absPath string) bool {
	return fileMatchesAny(absPath, r.excludes)
}

func run(pass *analysis.Pass, rules []compiledRule) (any, error) {
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
		if ignored[pos.Filename][pos.Line] {
			return
		}

		for _, rule := range rules {
			if rule.fileExcluded(pos.Filename) {
				continue
			}
			for _, p := range rule.patterns {
				if matches(obj, p) {
					if rule.name != "" {
						pass.Reportf(sel.Pos(),
							"forbidden reference to %s (rule: %s, pattern: %s)",
							qualifiedName(obj), rule.name, p.Raw)
					} else {
						pass.Reportf(sel.Pos(),
							"forbidden reference to %s (pattern: %s)",
							qualifiedName(obj), p.Raw)
					}
					return
				}
			}
		}
	})

	// Blank imports — `import _ "pkg"` — produce no SelectorExpr, so the
	// reference-based check above misses them. They're rare but real:
	// side-effect imports for init() functions (sql drivers, image
	// codecs, opentelemetry exporters). If the goal is "this package
	// can't appear in files outside the allowlist", a blank import
	// counts.
	//
	// Non-blank named or default imports either (a) reference some
	// identifier — caught above — or (b) compile-error as unused, so
	// they don't reach the linter. Hence we only need to handle the `_`
	// case here.
	for _, file := range pass.Files {
		for _, imp := range file.Imports {
			if imp.Name == nil || imp.Name.Name != "_" {
				continue
			}
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			pos := pass.Fset.Position(imp.Pos())
			if ignored[pos.Filename][pos.Line] {
				continue
			}
			for _, rule := range rules {
				if rule.fileExcluded(pos.Filename) {
					continue
				}
				matched := false
				for _, p := range rule.patterns {
					if importMatches(path, p) {
						if rule.name != "" {
							pass.Reportf(imp.Pos(),
								"forbidden blank import %q (rule: %s, pattern: %s)",
								path, rule.name, p.Raw)
						} else {
							pass.Reportf(imp.Pos(),
								"forbidden blank import %q (pattern: %s)",
								path, p.Raw)
						}
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
		}
	}

	return nil, nil
}

// importMatches reports whether a blank-import path is covered by the
// given pattern. For subtree patterns (`pkg/...`), the path matches if
// it equals pkg or is a subpackage of pkg. For all other patterns
// (pkg.Name, pkg.*, method receivers), the path matches if it equals
// pkg — importing the package is enough to count as "using" anything
// the pattern would flag.
func importMatches(path string, p Pattern) bool {
	if p.Receiver == "" && p.Name == "**" {
		return path == p.PkgPath || strings.HasPrefix(path, p.PkgPath+"/")
	}
	return path == p.PkgPath
}

// matches reports whether obj satisfies pattern p.
func matches(obj types.Object, p Pattern) bool {
	if obj.Pkg() == nil {
		return false
	}

	// Module subtree pattern: obj's package equals pkg OR is a
	// subpackage of pkg. By the time we see a cross-package
	// reference, the identifier is necessarily exported (otherwise
	// the type checker wouldn't have resolved it), so no extra
	// Exported() check is needed.
	if p.Receiver == "" && p.Name == "**" {
		return obj.Pkg().Path() == p.PkgPath ||
			strings.HasPrefix(obj.Pkg().Path(), p.PkgPath+"/")
	}

	if obj.Pkg().Path() != p.PkgPath {
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
