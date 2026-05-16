package method

import "net/http"

func F() {
	c := &http.Client{}
	_, _ = c.Get("https://example.com") // want `forbidden reference to net/http\.Get`
	_, _ = c.Do(nil)                    // want `forbidden reference to net/http\.Do`

	// Head is not in the forbid list — should not be flagged.
	_, _ = c.Head("https://example.com")
}
