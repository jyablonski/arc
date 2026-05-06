package boundary

import "net/http"

// HTTPDoer is the subset of net/http used by arc outbound callers.
// *http.Client satisfies this interface.
//
//go:generate go tool moq -rm -out http_doer_moq.go . HTTPDoer
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Compile-time check that the standard client implements HTTPDoer.
var _ HTTPDoer = (*http.Client)(nil)
