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
type Config struct {
	// Forbid is a list of patterns identifying APIs that may not be referenced.
	// Supported forms:
	//   pkg.Name            exact function/var (e.g. "os.Getenv")
	//   pkg.*               every exported identifier from the package
	//   (*pkg.Type).Method  pointer-receiver method (e.g. "(*net/http.Client).Do")
	//   (pkg.Type).Method   value-receiver method
	// Package paths use the full Go import path: "net/http", not "http".
	Forbid []string `yaml:"forbid" json:"forbid"`

	// ExcludeFiles is a list of doublestar globs. A file is exempted from
	// every rule if its absolute path matches, or if any path suffix
	// (relative-from-repo-root style) matches. Examples:
	//   internal/config/env.go
	//   **/*_test.go
	ExcludeFiles []string `yaml:"exclude_files" json:"exclude_files"`
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

// FileExcluded reports whether the given absolute file path is exempted.
// A pattern matches if it matches the absolute path, or any path suffix
// (so "internal/config/env.go" matches "/abs/repo/internal/config/env.go").
func (c Config) FileExcluded(absPath string) bool {
	p := filepath.ToSlash(absPath)
	for _, pat := range c.ExcludeFiles {
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
