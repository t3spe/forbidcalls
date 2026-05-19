package forbidcalls

import "testing"

func TestParsePatterns(t *testing.T) {
	tests := []struct {
		in   string
		ok   bool
		want Pattern
	}{
		{"os.Getenv", true, Pattern{PkgPath: "os", Name: "Getenv", Raw: "os.Getenv"}},
		{"os.*", true, Pattern{PkgPath: "os", Name: "*", Raw: "os.*"}},
		{
			"net/http.DefaultClient", true,
			Pattern{PkgPath: "net/http", Name: "DefaultClient", Raw: "net/http.DefaultClient"},
		},
		{
			"(*net/http.Client).Do", true,
			Pattern{PkgPath: "net/http", Name: "Do", Receiver: "*Client", Raw: "(*net/http.Client).Do"},
		},
		{
			"(net/http.Client).Do", true,
			Pattern{PkgPath: "net/http", Name: "Do", Receiver: "Client", Raw: "(net/http.Client).Do"},
		},
		{"invalid", false, Pattern{}},
		{"", false, Pattern{}},
		{"(Notpkg).Do", false, Pattern{}},
		{"(*os.).Do", false, Pattern{}},
	}
	for _, tc := range tests {
		ps, err := ParsePatterns([]string{tc.in})
		if (err == nil) != tc.ok {
			t.Errorf("ParsePatterns(%q) error=%v, want ok=%v", tc.in, err, tc.ok)
			continue
		}
		if tc.ok && ps[0] != tc.want {
			t.Errorf("ParsePatterns(%q) = %+v, want %+v", tc.in, ps[0], tc.want)
		}
	}
}

func TestNormalize(t *testing.T) {
	t.Run("legacy collapses to one rule", func(t *testing.T) {
		c := Config{
			Forbid:       []string{"os.Getenv"},
			ExcludeFiles: []string{"vendor/**"},
		}
		if err := c.Normalize(); err != nil {
			t.Fatal(err)
		}
		if len(c.Rules) != 1 {
			t.Fatalf("Rules len = %d, want 1", len(c.Rules))
		}
		if c.Forbid != nil || c.ExcludeFiles != nil {
			t.Errorf("legacy fields not cleared: Forbid=%v ExcludeFiles=%v", c.Forbid, c.ExcludeFiles)
		}
		if got, want := c.Rules[0].Forbid[0], "os.Getenv"; got != want {
			t.Errorf("Rules[0].Forbid[0] = %q, want %q", got, want)
		}
	})

	t.Run("rules-only stays untouched", func(t *testing.T) {
		c := Config{Rules: []Rule{{Name: "x", Forbid: []string{"os.Getenv"}}}}
		if err := c.Normalize(); err != nil {
			t.Fatal(err)
		}
		if len(c.Rules) != 1 || c.Rules[0].Name != "x" {
			t.Errorf("rules altered unexpectedly: %+v", c.Rules)
		}
	})

	t.Run("mixed config rejected", func(t *testing.T) {
		c := Config{
			Forbid: []string{"os.Getenv"},
			Rules:  []Rule{{Name: "x", Forbid: []string{"os.LookupEnv"}}},
		}
		if err := c.Normalize(); err == nil {
			t.Error("expected error from mixed config, got nil")
		}
	})

	t.Run("empty config is fine", func(t *testing.T) {
		c := Config{}
		if err := c.Normalize(); err != nil {
			t.Fatal(err)
		}
		if len(c.Rules) != 0 {
			t.Errorf("expected no rules, got %d", len(c.Rules))
		}
	})
}

func TestFileExcluded(t *testing.T) {
	cfg := Config{ExcludeFiles: []string{
		"internal/config/env.go",
		"**/*_test.go",
		"vendor/**",
	}}
	tests := []struct {
		path string
		want bool
	}{
		{"/abs/repo/internal/config/env.go", true},
		{"/abs/repo/internal/config/other.go", false},
		{"/abs/repo/pkg/bar_test.go", true},
		{"/abs/repo/vendor/foo/bar.go", true},
		{"/abs/repo/pkg/main.go", false},
		{"/internal/config/env.go", true},
	}
	for _, tc := range tests {
		if got := cfg.FileExcluded(tc.path); got != tc.want {
			t.Errorf("FileExcluded(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
