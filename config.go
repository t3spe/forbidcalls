package forbidcalls

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
)

// Config is the user-facing configuration for the analyzer. It can be loaded
// from YAML (standalone CLI) or decoded from golangci-lint plugin settings.
//
// Two schemas are supported. The recommended form is the `rules:` list,
// where each rule has its own Forbid + ExcludeFiles — handy when you
// want different deps scoped to different packages. The legacy schema
// (top-level Forbid + ExcludeFiles) collapses into a single anonymous
// rule. Mixing both forms is rejected. See Normalize.
type Config struct {
	// Rules holds independent forbid/exclude rule sets. First matching
	// rule wins for a given reference — once a reference matches some
	// rule's forbid pattern (with the file not in that rule's
	// exclude_files), a diagnostic fires and later rules are skipped
	// for that reference.
	Rules []Rule `yaml:"rules" json:"rules"`

	// Forbid is the legacy top-level pattern list. Normalize collapses
	// it into a single anonymous Rule when Rules is empty. See
	// Rule.Forbid for the pattern grammar.
	Forbid []string `yaml:"forbid" json:"forbid"`

	// ExcludeFiles is the legacy top-level exclusion list. See
	// Rule.ExcludeFiles for the glob grammar.
	ExcludeFiles []string `yaml:"exclude_files" json:"exclude_files"`
}

// Rule is one forbid/exclude pairing. Multiple rules let a single
// analyzer pass enforce per-dependency scoping rules without each
// rule's exclude list bleeding into the others.
type Rule struct {
	// Name is an optional label that appears in diagnostics
	// ("forbidden reference to X (rule: <name>, pattern: <p>)") so
	// failures point contributors at the responsible rule.
	Name string `yaml:"name" json:"name"`

	// Forbid is a list of patterns identifying APIs that may not be
	// referenced. Supported forms:
	//   pkg.Name            exact function/var (e.g. "os.Getenv")
	//   pkg.*               every exported identifier from the package
	//   (*pkg.Type).Method  pointer-receiver method (e.g. "(*net/http.Client).Do")
	//   (pkg.Type).Method   value-receiver method
	// Package paths use the full Go import path: "net/http", not "http".
	Forbid []string `yaml:"forbid" json:"forbid"`

	// ExcludeFiles is a list of doublestar globs. A file is exempted
	// from THIS rule (only) if its absolute path matches, or if any
	// path suffix (relative-from-repo-root style) matches. Examples:
	//   internal/config/env.go
	//   **/*_test.go
	//   vendor/**
	ExcludeFiles []string `yaml:"exclude_files" json:"exclude_files"`
}

// Normalize ensures the Config is in canonical (rules-only) form. If
// the caller used the legacy top-level fields, they're collapsed into
// a single anonymous Rule. Mixing both forms is an error.
func (c *Config) Normalize() error {
	hasLegacy := len(c.Forbid) > 0 || len(c.ExcludeFiles) > 0
	if len(c.Rules) > 0 && hasLegacy {
		return fmt.Errorf("config mixes both `rules:` and top-level `forbid`/`exclude_files`; pick one")
	}
	if len(c.Rules) == 0 && hasLegacy {
		c.Rules = []Rule{{
			Forbid:       c.Forbid,
			ExcludeFiles: c.ExcludeFiles,
		}}
		c.Forbid = nil
		c.ExcludeFiles = nil
	}
	return nil
}

// Pattern is a parsed Forbid entry.
type Pattern struct {
	PkgPath  string // import path, e.g. "os" or "net/http"
	Name     string // identifier, or "*" for any exported member
	Receiver string // empty for plain funcs; "Type" or "*Type" for methods
	Raw      string // original string, for diagnostics
}

// ParsePatterns converts the string forms into Pattern values.
func ParsePatterns(strs []string) ([]Pattern, error) {
	out := make([]Pattern, 0, len(strs))
	for _, s := range strs {
		p, err := parsePattern(s)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", s, err)
		}
		out = append(out, p)
	}
	return out, nil
}

func parsePattern(s string) (Pattern, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Pattern{}, fmt.Errorf("empty pattern")
	}

	// Module subtree form: pkg/... matches any exported identifier in pkg
	// or any subpackage of pkg. Useful for "no one outside this allowlist
	// may touch anything under golang.org/x/crypto" rules where the
	// subpackages aren't known in advance.
	if strings.HasSuffix(s, "/...") {
		pkgPath := strings.TrimSuffix(s, "/...")
		if pkgPath == "" {
			return Pattern{}, fmt.Errorf("missing package path before /...")
		}
		return Pattern{PkgPath: pkgPath, Name: "**", Raw: s}, nil
	}

	// Method form: (Recv).Method or (*Recv).Method where Recv = pkg.Type
	if strings.HasPrefix(s, "(") {
		closeParen := strings.Index(s, ")")
		if closeParen < 0 {
			return Pattern{}, fmt.Errorf("missing closing parenthesis")
		}
		if closeParen+2 > len(s) || s[closeParen+1] != '.' {
			return Pattern{}, fmt.Errorf("expected '.Method' after receiver")
		}
		recv := s[1:closeParen]
		name := s[closeParen+2:]
		if name == "" {
			return Pattern{}, fmt.Errorf("empty method name")
		}

		ptr := strings.HasPrefix(recv, "*")
		recvBody := strings.TrimPrefix(recv, "*")
		dot := strings.LastIndex(recvBody, ".")
		if dot < 0 {
			return Pattern{}, fmt.Errorf("receiver must be pkg.Type, got %q", recv)
		}
		pkgPath := recvBody[:dot]
		typeName := recvBody[dot+1:]
		if pkgPath == "" || typeName == "" {
			return Pattern{}, fmt.Errorf("receiver must be pkg.Type, got %q", recv)
		}
		receiverNorm := typeName
		if ptr {
			receiverNorm = "*" + typeName
		}
		return Pattern{
			PkgPath:  pkgPath,
			Name:     name,
			Receiver: receiverNorm,
			Raw:      s,
		}, nil
	}

	// Function form: pkg.Name or pkg.*
	dot := strings.LastIndex(s, ".")
	if dot < 0 {
		return Pattern{}, fmt.Errorf("expected pkg.Name or pkg.*")
	}
	pkgPath := s[:dot]
	name := s[dot+1:]
	if pkgPath == "" || name == "" {
		return Pattern{}, fmt.Errorf("expected pkg.Name or pkg.*")
	}
	return Pattern{PkgPath: pkgPath, Name: name, Raw: s}, nil
}

// LoadConfig reads YAML from the given path.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &c, nil
}

// FileExcluded reports whether the given absolute file path is exempted
// per Config.ExcludeFiles. Kept for legacy callers that constructed a
// Config with the top-level fields only; new code should compose a
// Rule and call Rule.FileExcluded.
func (c Config) FileExcluded(absPath string) bool {
	return fileMatchesAny(absPath, c.ExcludeFiles)
}

// FileExcluded reports whether the given absolute file path is exempted
// by this rule.
func (r Rule) FileExcluded(absPath string) bool {
	return fileMatchesAny(absPath, r.ExcludeFiles)
}

// fileMatchesAny returns true if path (treated as both the absolute
// path and any of its trailing-path suffixes) matches at least one of
// the given doublestar globs. The suffix sweep means a glob like
// "internal/config/env.go" matches "/abs/repo/internal/config/env.go"
// without the caller knowing the repo root.
func fileMatchesAny(absPath string, patterns []string) bool {
	p := filepath.ToSlash(absPath)
	for _, pat := range patterns {
		if matchGlob(pat, p) {
			return true
		}
		parts := strings.Split(p, "/")
		for i := 1; i < len(parts); i++ {
			if matchGlob(pat, strings.Join(parts[i:], "/")) {
				return true
			}
		}
	}
	return false
}

func matchGlob(pattern, path string) bool {
	ok, err := doublestar.PathMatch(pattern, path)
	return err == nil && ok
}
