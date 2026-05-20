package blankimport

import (
	_ "net/http/pprof" // want `forbidden blank import "net/http/pprof" \(pattern: net/http/\.\.\.\)`
	_ "syscall"        // want `forbidden blank import "syscall" \(pattern: syscall\.\*\)`

	// Non-blank imports of the same packages are caught by the
	// SelectorExpr check (not this case). A blank import of an
	// unrelated package doesn't fire.
	_ "unicode"
)
