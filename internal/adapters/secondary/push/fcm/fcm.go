// Package fcm sends push notifications through the Firebase Cloud Messaging HTTP v1 API.
package fcm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/push"
)

const (
	scope           = "https://www.googleapis.com/auth/firebase.messaging"
	defaultEndpoint = "https://fcm.googleapis.com"
	// requestTimeout bounds one FCM call, including token refresh.
	requestTimeout = 10 * time.Second
)

// Config builds a pusher. Endpoint and HTTPClient exist so tests can point it at an httptest server;
// production code uses NewFromFile.
type Config struct {
	ProjectID  string
	Endpoint   string       // default https://fcm.googleapis.com
	HTTPClient *http.Client // must add authorization; default http.DefaultClient
}

type pusher struct {
	url    string
	client *http.Client
}

func New(c Config) push.Pusher {
	if c.Endpoint == "" {
		c.Endpoint = defaultEndpoint
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: requestTimeout}
	}
	return &pusher{url: c.Endpoint + "/v1/projects/" + c.ProjectID + "/messages:send", client: c.HTTPClient}
}

// NewFromFile reads a service-account JSON file and returns a pusher whose requests carry its OAuth2 token.
func NewFromFile(_ context.Context, credentialsFile string) (push.Pusher, error) {
	raw, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("fcm.NewFromFile read credentials: %w", err)
	}
	// The token source keeps this context for every refresh, so it must outlive the caller's startup ctx;
	// its own HTTP client bounds the token endpoint call.
	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{Timeout: requestTimeout})
	creds, err := google.CredentialsFromJSON(tokenCtx, raw, scope)
	if err != nil {
		return nil, fmt.Errorf("fcm.NewFromFile parse credentials: %w", err)
	}
	if creds.ProjectID == "" {
		return nil, fmt.Errorf("fcm.NewFromFile: credentials file has no project_id")
	}
	client := newAuthClient(creds.TokenSource)
	client.Timeout = requestTimeout
	return New(Config{ProjectID: creds.ProjectID, HTTPClient: client}), nil
}

type message struct {
	Token        string            `json:"token"`
	Notification map[string]string `json:"notification"`
	Data         map[string]string `json:"data"`
}

func (p *pusher) Send(ctx context.Context, token string, n domain.Notification) (bool, error) {
	data := make(map[string]string, len(n.Data)+2)
	for k, v := range n.Data {
		data[k] = v
	}
	data["kind"] = n.Kind
	data["notificationId"] = strconv.FormatInt(n.ID, 10)
	body, err := json.Marshal(map[string]message{"message": {
		Token:        token,
		Notification: map[string]string{"title": n.Title, "body": n.Body},
		Data:         data,
	}})
	if err != nil {
		return false, fmt.Errorf("fcm.Send marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url, bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("fcm.Send request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := p.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("fcm.Send do: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	if res.StatusCode/100 == 2 {
		return false, nil
	}
	var e struct {
		Error struct {
			Status  string `json:"status"`
			Message string `json:"message"`
			Details []struct {
				ErrorCode string `json:"errorCode"`
			} `json:"details"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &e)
	// A bare 404 can mean a wrong project or endpoint, so only the explicit UNREGISTERED code prunes a token.
	for _, d := range e.Error.Details {
		if d.ErrorCode == "UNREGISTERED" {
			return true, nil
		}
	}
	// The message is Google's, not ours, and never includes the request body; the token is not echoed.
	return false, fmt.Errorf("fcm.Send: status %d %s", res.StatusCode, e.Error.Status)
}
