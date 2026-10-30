package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// slowPost streams 5 chunks 100ms apart (about 500ms in all) to a server whose ReadTimeout is 200ms.
func slowPost(t *testing.T, h http.Handler) (int, error) {
	t.Helper()
	srv := httptest.NewUnstartedServer(Logger(h)) // through statusRecorder, like production
	srv.Config.ReadTimeout = 200 * time.Millisecond
	srv.Config.WriteTimeout = 200 * time.Millisecond
	srv.Start()
	defer srv.Close()

	pr, pw := io.Pipe()
	go func() {
		for i := 0; i < 5; i++ {
			time.Sleep(100 * time.Millisecond)
			if _, err := pw.Write([]byte("0123456789")); err != nil {
				return
			}
		}
		_ = pw.Close()
	}()
	res, err := http.Post(srv.URL, "application/octet-stream", pr)
	if err != nil {
		return 0, err
	}
	defer func() { _ = res.Body.Close() }()
	return res.StatusCode, nil
}

func readAll(w http.ResponseWriter, r *http.Request) {
	b, err := io.ReadAll(r.Body)
	if err != nil || len(b) != 50 {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func TestExtendDeadline_SlowUploadOutlivesServerTimeout(t *testing.T) {
	if status, err := slowPost(t, ExtendDeadline(5*time.Second)(http.HandlerFunc(readAll))); err != nil || status != http.StatusOK {
		t.Fatalf("extended: status %d err %v", status, err)
	}
	// Control: the same request without the extension is cut by the server ReadTimeout.
	if status, err := slowPost(t, http.HandlerFunc(readAll)); err == nil && status == http.StatusOK {
		t.Fatal("control request unexpectedly survived the 200ms ReadTimeout")
	}
}
