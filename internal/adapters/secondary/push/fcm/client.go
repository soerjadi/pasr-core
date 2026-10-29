package fcm

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
)

// newAuthClient is deliberately not tied to the caller's context: the token source keeps refreshing for the
// life of the process, and a cancelled startup context would otherwise break every later refresh.
func newAuthClient(ts oauth2.TokenSource) *http.Client {
	return oauth2.NewClient(context.Background(), ts)
}
