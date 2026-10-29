// Package local stores objects on the local filesystem and serves them over HTTP.
// All file access goes through os.Root, so a key can never escape the storage directory.
package local

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/oemah/nusantara/core/internal/core/ports/secondary/storage"
)

type Storage struct {
	root    *os.Root
	baseURL string
}

var _ storage.ObjectStorage = (*Storage)(nil)

// New creates the directory if needed. baseURL is the public URL prefix that maps to Serve.
func New(dir, baseURL string) (*Storage, error) {
	if dir == "" {
		return nil, errors.New("local storage: dir is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("local storage: create dir: %w", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("local storage: open root: %w", err)
	}
	return &Storage{root: root, baseURL: strings.TrimRight(baseURL, "/")}, nil
}

func (s *Storage) Close() error { return s.root.Close() }

// validKey accepts only clean, relative, slash-separated keys.
func validKey(key string) bool {
	return key != "" && fs.ValidPath(key) && key != "." && path.Clean(key) == key
}

func (s *Storage) Put(_ context.Context, key, _ string, r io.Reader) error {
	if !validKey(key) {
		return fmt.Errorf("local storage: invalid key")
	}
	if dir := path.Dir(key); dir != "." {
		if err := s.root.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("local storage: mkdir: %w", err)
		}
	}
	f, err := s.root.OpenFile(key, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("local storage: create object: %w", err)
	}
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		_ = s.root.Remove(key)
		return fmt.Errorf("local storage: write object: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = s.root.Remove(key)
		return fmt.Errorf("local storage: close object: %w", err)
	}
	return nil
}

func (s *Storage) Delete(_ context.Context, key string) error {
	if !validKey(key) {
		return fmt.Errorf("local storage: invalid key")
	}
	if err := s.root.Remove(key); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("local storage: delete object: %w", err)
	}
	return nil
}

func (s *Storage) URL(key string) string { return s.baseURL + "/" + key }

// Serve writes the object to w, or 404 when the key is invalid or missing.
func (s *Storage) Serve(w http.ResponseWriter, r *http.Request, key string) {
	if !validKey(key) {
		http.NotFound(w, r)
		return
	}
	f, err := s.root.Open(key)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}
	ct := mime.TypeByExtension(path.Ext(key))
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(w, r, "", st.ModTime(), f)
}
