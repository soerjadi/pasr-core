package testapp

import (
	"context"
	"io"
	"sort"
	"sync"
)

// MemStorage is an in-memory storage.ObjectStorage. testapp uses it instead of the local
// filesystem adapter (which has its own tests) so tests leave no files behind and can inspect
// the stored bytes.
type MemStorage struct {
	mu   sync.Mutex
	objs map[string][]byte
	ct   map[string]string
}

func NewMemStorage() *MemStorage {
	return &MemStorage{objs: map[string][]byte{}, ct: map[string]string{}}
}

const memBaseURL = "http://media.test/"

func (m *MemStorage) Put(_ context.Context, key, contentType string, r io.Reader) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objs[key] = b
	m.ct[key] = contentType
	return nil
}

func (m *MemStorage) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objs, key)
	delete(m.ct, key)
	return nil
}

func (m *MemStorage) URL(key string) string { return memBaseURL + key }

// KeyFromURL reverses URL; ok is false for foreign URLs.
func (m *MemStorage) KeyFromURL(u string) (string, bool) {
	if len(u) <= len(memBaseURL) || u[:len(memBaseURL)] != memBaseURL {
		return "", false
	}
	return u[len(memBaseURL):], true
}

// Get returns a copy of the object and its content type.
func (m *MemStorage) Get(key string) (data []byte, contentType string, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objs[key]
	return append([]byte(nil), b...), m.ct[key], ok
}

// Keys lists the stored keys in order.
func (m *MemStorage) Keys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]string, 0, len(m.objs))
	for k := range m.objs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
