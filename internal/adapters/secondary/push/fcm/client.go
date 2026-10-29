package fcm

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
)

func newAuthClient(ctx context.Context, ts oauth2.TokenSource) *http.Client {
	return oauth2.NewClient(ctx, ts)
}
