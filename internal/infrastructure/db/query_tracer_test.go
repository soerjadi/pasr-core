package db

import (
	"testing"

	"github.com/oemah/nusantara/core/internal/infrastructure/config"
)

func TestTruncateSQL(t *testing.T) {
	if got := truncateSQL("SELECT 1", 0); got != "SELECT 1" {
		t.Errorf("0 means no truncation, got %q", got)
	}
	if got := truncateSQL("SELECT 1", 6); got != "SELECT" {
		t.Errorf("got %q", got)
	}
	if got := truncateSQL("SELECT", 10); got != "SELECT" {
		t.Errorf("got %q", got)
	}
}

func TestNewQueryTracer(t *testing.T) {
	if newQueryTracer(config.Database{}) != nil {
		t.Error("disabled must return nil")
	}
	if _, ok := newQueryTracer(config.Database{TraceEnabled: true, TraceMaxQueryLen: 100}).(*queryTruncatingTracer); !ok {
		t.Error("max length must wrap in truncating tracer")
	}
	if _, ok := newQueryTracer(config.Database{TraceEnabled: true}).(*queryTruncatingTracer); ok {
		t.Error("no max length must not wrap")
	}
}
