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
