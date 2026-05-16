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
