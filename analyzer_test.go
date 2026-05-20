package forbidcalls_test

import (
	"testing"

	"github.com/t3spe/forbidcalls"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestBasic(t *testing.T) {
	a, err := forbidcalls.NewAnalyzer(forbidcalls.Config{
		Forbid: []string{"os.Getenv", "os.LookupEnv", "os.Environ"},
	})
	if err != nil {
		t.Fatal(err)
	}
	analysistest.Run(t, analysistest.TestData(), a, "basic")
}

func TestWildcard(t *testing.T) {
	a, err := forbidcalls.NewAnalyzer(forbidcalls.Config{
		Forbid: []string{"syscall.*"},
	})
	if err != nil {
		t.Fatal(err)
	}
	analysistest.Run(t, analysistest.TestData(), a, "wildcard")
}

func TestMethodReceiver(t *testing.T) {
	a, err := forbidcalls.NewAnalyzer(forbidcalls.Config{
		Forbid: []string{
			"(*net/http.Client).Get",
			"(*net/http.Client).Do",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	analysistest.Run(t, analysistest.TestData(), a, "method")
}

func TestIgnoreDirective(t *testing.T) {
	a, err := forbidcalls.NewAnalyzer(forbidcalls.Config{
		Forbid: []string{"os.Getenv"},
	})
	if err != nil {
		t.Fatal(err)
	}
	analysistest.Run(t, analysistest.TestData(), a, "ignored")
}

// TestModuleSubtree verifies the `pkg/...` form matches identifiers
// in pkg itself plus any subpackage.
func TestModuleSubtree(t *testing.T) {
	a, err := forbidcalls.NewAnalyzer(forbidcalls.Config{
		Forbid: []string{"net/http/..."},
	})
	if err != nil {
		t.Fatal(err)
	}
	analysistest.Run(t, analysistest.TestData(), a, "subtree")
}

// TestMultipleRules verifies per-rule exclude_files: two rules each
// scoped to a different package exclude the other's files, so each
// only fires inside its own scope.
func TestMultipleRules(t *testing.T) {
	a, err := forbidcalls.NewAnalyzer(forbidcalls.Config{
		Rules: []forbidcalls.Rule{
			{
				Name:         "no-os-in-b",
				Forbid:       []string{"os.Getenv"},
				ExcludeFiles: []string{"**/multirule/a/**"},
			},
			{
				Name:         "no-syscall-in-a",
				Forbid:       []string{"syscall.*"},
				ExcludeFiles: []string{"**/multirule/b/**"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	analysistest.Run(t, analysistest.TestData(), a, "multirule/a", "multirule/b")
}

// TestMixedConfigRejected ensures we surface a clear error when callers
// combine the legacy top-level fields with the new rules schema —
// otherwise it's easy to think both apply and silently get one ignored.
func TestMixedConfigRejected(t *testing.T) {
	_, err := forbidcalls.NewAnalyzer(forbidcalls.Config{
		Forbid: []string{"os.Getenv"},
		Rules: []forbidcalls.Rule{
			{Name: "x", Forbid: []string{"os.LookupEnv"}},
		},
	})
	if err == nil {
		t.Fatal("expected error from mixed config, got nil")
	}
}
