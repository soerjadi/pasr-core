package rest

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLimit(t *testing.T) {
	for _, tc := range []struct {
		query string
		want  int
		ok    bool
	}{
		{"", 0, true}, {"limit=1", 1, true}, {"limit=50", 50, true}, {"limit=99999", 1000, true},
		{"limit=0", 0, false}, {"limit=-1", 0, false}, {"limit=abc", 0, false}, {"limit=1.5", 0, false},
	} {
		rec := httptest.NewRecorder()
		got, ok := Limit(rec, httptest.NewRequest(http.MethodGet, "/x?"+tc.query, nil))
		if got != tc.want || ok != tc.ok {
			t.Errorf("%q: got %d,%v want %d,%v", tc.query, got, ok, tc.want, tc.ok)
		}
		if !ok && rec.Code != http.StatusBadRequest {
			t.Errorf("%q: status %d", tc.query, rec.Code)
		}
	}
}
