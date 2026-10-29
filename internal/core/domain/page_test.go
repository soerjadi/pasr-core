package domain

import (
	"errors"
	"testing"
)

func ident(n int64) int64 { return n }

func TestBuildPage_LastPageHasNullCursor(t *testing.T) {
	p := BuildPage([]int64{5, 4}, 3, ident)
	if len(p.Items) != 2 || p.NextCursor != nil {
		t.Fatalf("got %+v", p)
	}
	empty := BuildPage[int64](nil, 20, ident)
	if empty.Items == nil || len(empty.Items) != 0 || empty.NextCursor != nil {
		t.Fatalf("empty page: %+v", empty)
	}
}

func TestBuildPage_MoreRowsSetsCursor(t *testing.T) {
	p := BuildPage([]int64{9, 8, 7}, 2, ident)
	if len(p.Items) != 2 || p.NextCursor == nil {
		t.Fatalf("got %+v", p)
	}
	id, err := DecodeCursor(*p.NextCursor)
	if err != nil || id != 8 {
		t.Fatalf("cursor id=%d err=%v, want 8", id, err)
	}
}

func TestDecodeCursor_Malformed(t *testing.T) {
	if id, err := DecodeCursor(""); err != nil || id != 0 {
		t.Fatalf("empty: %d %v", id, err)
	}
	for _, s := range []string{"!!!", "bm90LWEtbnVtYmVy", "LTU"} {
		if _, err := DecodeCursor(s); !errors.Is(err, ErrValidationFailed) {
			t.Fatalf("%q: err=%v", s, err)
		}
	}
	if id, err := DecodeCursor(EncodeCursor(42)); err != nil || id != 42 {
		t.Fatalf("roundtrip: %d %v", id, err)
	}
}

func TestClampLimit(t *testing.T) {
	for in, want := range map[int]int{-1: 20, 0: 20, 1: 1, 50: 50, 51: 50, 1000: 50} {
		if got := ClampLimit(in); got != want {
			t.Errorf("ClampLimit(%d)=%d want %d", in, got, want)
		}
	}
}
