// Package contract checks API responses against the shared JSON fixtures.
package contract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

// fixturesDir resolves docs/api/fixtures relative to this source file.
func fixturesDir() string {
	_, file, _, _ := runtime.Caller(0)
	// <repo>/core/internal/testutil/contract/shape.go -> <repo>/docs/api/fixtures
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
	return filepath.Join(root, "docs", "api", "fixtures")
}

// AssertShape fails t unless got has the same JSON shape as the operation's fixture:
// equal recursive key sets (fixture keys with a null value may be absent), equal value kinds,
// and arrays compared on their first element when both sides are non-empty.
func AssertShape(t testing.TB, operationID string, got []byte) {
	t.Helper()
	path := filepath.Join(fixturesDir(), operationID+".json")
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("contract: read fixture %s: %v", path, err)
		return
	}
	compare(t, "$", want, got)
}

func compare(t testing.TB, path string, want, got []byte) {
	t.Helper()
	var w, g any
	if err := json.Unmarshal(want, &w); err != nil {
		t.Errorf("contract: %s: invalid fixture JSON: %v", path, err)
		return
	}
	if err := json.Unmarshal(got, &g); err != nil {
		t.Errorf("contract: %s: invalid response JSON: %v", path, err)
		return
	}
	walk(t, path, w, g)
}

func kind(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case float64:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return fmt.Sprintf("%T", v)
}

func walk(t testing.TB, path string, w, g any) {
	t.Helper()
	if w == nil {
		return // fixture null: kind unknown, anything goes
	}
	if kw, kg := kind(w), kind(g); kw != kg {
		t.Errorf("contract: %s: kind %s, want %s", path, kg, kw)
		return
	}
	switch wv := w.(type) {
	case map[string]any:
		gv := g.(map[string]any)
		for _, k := range sortedKeys(wv) {
			gk, ok := gv[k]
			if !ok {
				if wv[k] != nil {
					t.Errorf("contract: %s.%s: missing key", path, k)
				}
				continue
			}
			walk(t, path+"."+k, wv[k], gk)
		}
		for _, k := range sortedKeys(gv) {
			if _, ok := wv[k]; !ok {
				t.Errorf("contract: %s.%s: unexpected key", path, k)
			}
		}
	case []any:
		gv := g.([]any)
		if len(wv) > 0 && len(gv) > 0 {
			walk(t, path+"[0]", wv[0], gv[0])
		}
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
