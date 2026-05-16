package basic

import "os"

func F() {
	_ = os.Getenv("HOME")    // want `forbidden reference to os\.Getenv`
	_, _ = os.LookupEnv("X") // want `forbidden reference to os\.LookupEnv`
	_ = os.Environ()         // want `forbidden reference to os\.Environ`

	// Other os identifiers are fine when only specific names are forbidden.
	_ = os.Stdout

	// Value capture (alias) is still a reference.
	f := os.Getenv // want `forbidden reference to os\.Getenv`
	_ = f
}
