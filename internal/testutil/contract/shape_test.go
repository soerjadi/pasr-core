package contract

import (
	"fmt"
	"testing"
)

// recorder is a testing.TB stand-in that records failures instead of failing the real test.
type recorder struct {
	testing.TB
	failed bool
	msgs   []string
}

func (r *recorder) Helper() {}

func (r *recorder) Errorf(format string, args ...any) {
	r.failed = true
	r.msgs = append(r.msgs, fmt.Sprintf(format, args...))
}

func (r *recorder) Fatalf(format string, args ...any) {
	r.Errorf(format, args...)
}

func assertOutcome(t *testing.T, want bool, body string) *recorder {
	t.Helper()
	r := &recorder{TB: t}
	AssertShape(r, "listCategories", []byte(body))
	if r.failed != want {
		t.Fatalf("expected failure=%v, got failure=%v (%v)", want, r.failed, r.msgs)
	}
	return r
}

func TestAssertShape_EqualKeysPass(t *testing.T) {
	assertOutcome(t, false, `{"items":[{"id":7,"name":"x","iconKey":"y"}]}`)
}

func TestAssertShape_EmptyArrayPasses(t *testing.T) {
	assertOutcome(t, false, `{"items":[]}`)
}

func TestAssertShape_ExtraKeyFails(t *testing.T) {
	assertOutcome(t, true, `{"items":[{"id":7,"name":"x","iconKey":"y","extra":1}]}`)
}

func TestAssertShape_MissingKeyFails(t *testing.T) {
	assertOutcome(t, true, `{"items":[{"id":7,"name":"x"}]}`)
}

func TestAssertShape_KindMismatchFails(t *testing.T) {
	assertOutcome(t, true, `{"items":[{"id":"7","name":"x","iconKey":"y"}]}`)
}

func TestAssertShape_NullFixtureKeyMayBeAbsent(t *testing.T) {
	// getMe fixture has null-valued keys; confirm via a dedicated helper on raw JSON.
	r := &recorder{TB: t}
	compare(r, "$", []byte(`{"a":1,"b":null}`), []byte(`{"a":2}`))
	if r.failed {
		t.Fatalf("absent null key should pass: %v", r.msgs)
	}
	r = &recorder{TB: t}
	compare(r, "$", []byte(`{"a":1,"b":null}`), []byte(`{"a":2,"b":"anything"}`))
	if r.failed {
		t.Fatalf("present null-fixture key should pass: %v", r.msgs)
	}
	r = &recorder{TB: t}
	compare(r, "$", []byte(`{"a":1,"b":2}`), []byte(`{"a":2}`))
	if !r.failed {
		t.Fatal("absent non-null key should fail")
	}
}
