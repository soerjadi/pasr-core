package s3

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestS3_PutDeleteURL(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path+" "+r.Header.Get("Content-Type"))
		mu.Unlock()
		w.Header().Set("ETag", `"abc"`)
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	s, err := New(Config{Endpoint: srv.URL, Bucket: "media", AccessKey: "k", SecretKey: "s", PublicBaseURL: "https://cdn.example.com/"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := s.Put(ctx, "images/a.jpg", "image/jpeg", bytes.NewReader([]byte("data"))); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, "images/a.jpg"); err != nil {
		t.Fatal(err)
	}
	if u := s.URL("images/a.jpg"); u != "https://cdn.example.com/images/a.jpg" {
		t.Fatalf("URL = %q", u)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(calls) < 2 || calls[0] != "PUT /media/images/a.jpg image/jpeg" || calls[len(calls)-1][:6] != "DELETE" {
		t.Fatalf("calls = %v", calls)
	}
}
