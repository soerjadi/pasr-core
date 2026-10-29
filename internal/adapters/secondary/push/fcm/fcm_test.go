package fcm_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oemah/nusantara/core/internal/adapters/secondary/push/fcm"
	"github.com/oemah/nusantara/core/internal/core/domain"
)

func newPusher(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

var note = domain.Notification{ID: 7, UserID: 1, Kind: domain.NotifOrderReady, Title: "Pesanan siap diambil", Body: "Siap",
	Data: map[string]string{"orderNumber": "NM-1"}}

func TestFCM_BuildsV1RequestWithData(t *testing.T) {
	var path string
	var got map[string]map[string]any
	srv := newPusher(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		_, _ = w.Write([]byte(`{"name":"projects/p/messages/1"}`))
	})
	p := fcm.New(fcm.Config{ProjectID: "demo", Endpoint: srv.URL, HTTPClient: srv.Client()})
	unreg, err := p.Send(context.Background(), "device-token", note)
	if err != nil || unreg {
		t.Fatalf("unreg=%v err=%v", unreg, err)
	}
	if path != "/v1/projects/demo/messages:send" {
		t.Fatalf("path = %q", path)
	}
	m := got["message"]
	if m["token"] != "device-token" {
		t.Fatalf("token = %v", m["token"])
	}
	n := m["notification"].(map[string]any)
	if n["title"] != note.Title || n["body"] != note.Body {
		t.Fatalf("notification = %v", n)
	}
	d := m["data"].(map[string]any)
	if d["orderNumber"] != "NM-1" || d["kind"] != domain.NotifOrderReady || d["notificationId"] != "7" {
		t.Fatalf("data = %v", d)
	}
}

func TestFCM_UnregisteredAndErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		unreg  bool
		err    bool
	}{
		{"404", 404, `{"error":{"status":"NOT_FOUND"}}`, true, false},
		{"unregistered code", 400, `{"error":{"status":"INVALID_ARGUMENT","details":[{"errorCode":"UNREGISTERED"}]}}`, true, false},
		{"server error", 500, `{"error":{"message":"oops"}}`, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := newPusher(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(c.status)
				_, _ = w.Write([]byte(c.body))
			})
			p := fcm.New(fcm.Config{ProjectID: "demo", Endpoint: srv.URL, HTTPClient: srv.Client()})
			unreg, err := p.Send(context.Background(), "secret-device-token", note)
			if unreg != c.unreg || (err != nil) != c.err {
				t.Fatalf("unreg=%v err=%v", unreg, err)
			}
			if err != nil && contains(err.Error(), "secret-device-token") {
				t.Fatalf("error leaks token: %v", err)
			}
		})
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
