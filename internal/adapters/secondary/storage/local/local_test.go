package local

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStorage_PutURLDelete(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir, "http://localhost:8080/media/")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	if err := s.Put(ctx, "images/ab/cd.jpg", "image/jpeg", bytes.NewReader([]byte("data"))); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "images", "ab", "cd.jpg"))
	if err != nil || string(got) != "data" {
		t.Fatalf("file = %q, %v", got, err)
	}
	if u := s.URL("images/ab/cd.jpg"); u != "http://localhost:8080/media/images/ab/cd.jpg" {
		t.Fatalf("URL = %q", u)
	}

	rec := httptest.NewRecorder()
	s.Serve(rec, httptest.NewRequest(http.MethodGet, "/media/x", nil), "images/ab/cd.jpg")
	body, _ := io.ReadAll(rec.Body)
	if rec.Code != 200 || string(body) != "data" || rec.Header().Get("Content-Type") != "image/jpeg" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("serve = %d %q %v", rec.Code, body, rec.Header())
	}

	if err := s.Delete(ctx, "images/ab/cd.jpg"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "images", "ab", "cd.jpg")); !os.IsNotExist(err) {
		t.Fatalf("file still exists: %v", err)
	}
	if err := s.Delete(ctx, "images/ab/cd.jpg"); err != nil {
		t.Fatalf("deleting a missing object must succeed: %v", err)
	}
}

func TestLocalStorage_RejectsEscapingKeys(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "store")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := New(dir, "http://x/media")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	for _, key := range []string{"../evil.jpg", "/etc/passwd", "a/../../evil.jpg", ""} {
		if err := s.Put(context.Background(), key, "image/jpeg", bytes.NewReader([]byte("x"))); err == nil {
			t.Errorf("Put(%q) succeeded", key)
		}
	}
	if _, err := os.Stat(filepath.Join(parent, "evil.jpg")); err == nil {
		t.Fatal("file escaped the storage root")
	}

	rec := httptest.NewRecorder()
	s.Serve(rec, httptest.NewRequest(http.MethodGet, "/media/..%2fx", nil), "../store/../x")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("serve escaping key = %d", rec.Code)
	}
}
