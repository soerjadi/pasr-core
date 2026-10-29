package media

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/oemah/nusantara/core/internal/adapters/secondary/storage/local"
)

func TestServe_LocalFilesOnly(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "store")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := local.New(dir, "http://x/media")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Put(context.Background(), "images/a-s.jpg", "image/jpeg", bytes.NewReader([]byte("jpegdata"))); err != nil {
		t.Fatal(err)
	}

	h := NewHandler(nil, 1, st)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /media/{key...}", h.Serve)

	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}
	if rec := get("/media/images/a-s.jpg"); rec.Code != 200 || rec.Body.String() != "jpegdata" || rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("hit = %d %q %v", rec.Code, rec.Body.String(), rec.Header())
	}
	for _, p := range []string{"/media/images/nope.jpg", "/media/images", "/media/images/", "/media/../secret.txt", "/media/%2e%2e/secret.txt", "/media/..%2fsecret.txt"} {
		if rec := get(p); rec.Code == 200 {
			t.Errorf("%s served: %q", p, rec.Body.String())
		}
	}
}
