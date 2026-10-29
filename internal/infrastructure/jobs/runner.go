package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/oemahdev/logger"
)

type job struct {
	name  string
	every time.Duration
	fn    func(ctx context.Context) error
}

// Runner runs periodic jobs. Each job has its own goroutine and runs sequentially,
// so a job never overlaps with itself.
type Runner struct {
	mu     sync.Mutex
	jobs   []job
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewRunner() *Runner { return &Runner{} }

func (r *Runner) Add(name string, every time.Duration, fn func(ctx context.Context) error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs = append(r.jobs, job{name: name, every: every, fn: fn})
}

// Start launches all added jobs. Calling it twice is a no-op.
func (r *Runner) Start(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		return
	}
	ctx, r.cancel = context.WithCancel(ctx)
	for _, j := range r.jobs {
		r.wg.Add(1)
		go r.loop(ctx, j)
	}
}

// Stop cancels the jobs and waits for running ones to finish or for ctx to expire.
func (r *Runner) Stop(ctx context.Context) error {
	r.mu.Lock()
	cancel := r.cancel
	r.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	done := make(chan struct{})
	go func() { r.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("jobs.Stop: %w", ctx.Err())
	}
}

func (r *Runner) loop(ctx context.Context, j job) {
	defer r.wg.Done()
	t := time.NewTicker(j.every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			run(ctx, j)
		}
	}
}

func run(ctx context.Context, j job) {
	defer func() {
		if p := recover(); p != nil {
			log.ErrorCtx(ctx, "job panicked", log.Field("job", j.name), log.Field("panic", fmt.Sprint(p)))
		}
	}()
	if err := j.fn(ctx); err != nil {
		log.ErrorCtx(ctx, "job failed", log.Field("job", j.name), log.Field("error", err))
	}
}
