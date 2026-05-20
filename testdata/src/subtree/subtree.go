package subtree

import (
	"net/http"
	"net/http/httptest"
)

// Forbid pattern `net/http/...` matches both net/http and its
// subpackages, so both references fire.
func F() {
	_ = http.MethodGet                             // want `forbidden reference to net/http\.MethodGet \(pattern: net/http/\.\.\.\)`
	_ = httptest.NewRecorder                       // want `forbidden reference to net/http/httptest\.NewRecorder \(pattern: net/http/\.\.\.\)`
	_ = http.StatusOK                              // want `forbidden reference to net/http\.StatusOK \(pattern: net/http/\.\.\.\)`
	_ = httptest.NewServer                         // want `forbidden reference to net/http/httptest\.NewServer \(pattern: net/http/\.\.\.\)`
}
