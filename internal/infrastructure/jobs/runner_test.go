package jobs

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func eventually(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met in time")
}

func TestRunner_RunsOnIntervalAndStops(t *testing.T) {
	var n atomic.Int32
	r := NewRunner()
	r.Add("tick", 10*time.Millisecond, func(context.Context) error { n.Add(1); return nil })
	r.Start(context.Background())
	eventually(t, func() bool { return n.Load() >= 3 })
	if err := r.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	after := n.Load()
	time.Sleep(50 * time.Millisecond)
	if n.Load() != after {
		t.Fatal("job ran after Stop")
	}
}

func TestRunner_DoesNotOverlap(t *testing.T) {
	var running, maxRunning, calls atomic.Int32
	r := NewRunner()
	r.Add("slow", 5*time.Millisecond, func(context.Context) error {
		c := running.Add(1)
		if c > maxRunning.Load() {
			maxRunning.Store(c)
		}
		calls.Add(1)
		time.Sleep(30 * time.Millisecond)
		running.Add(-1)
		return nil
	})
	r.Start(context.Background())
	eventually(t, func() bool { return calls.Load() >= 3 })
	if err := r.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if maxRunning.Load() != 1 {
		t.Fatalf("max concurrent = %d", maxRunning.Load())
	}
	if running.Load() != 0 {
		t.Fatal("Stop returned before running job finished")
	}
}

func TestRunner_RecoversPanic(t *testing.T) {
	var n atomic.Int32
	r := NewRunner()
	r.Add("panicky", 5*time.Millisecond, func(context.Context) error {
		if n.Add(1) == 1 {
			panic("boom")
		}
		return errors.New("logged, not fatal")
	})
	r.Start(context.Background())
	eventually(t, func() bool { return n.Load() >= 3 })
	if err := r.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}
