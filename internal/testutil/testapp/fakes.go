package testapp

import (
	"context"
	"sync"
	"time"
)

// FakeClock is a controllable domain.Clock.
type FakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func NewFakeClock(now time.Time) *FakeClock { return &FakeClock{now: now} }

func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// FakeOTPSender records the latest code sent to each phone.
type FakeOTPSender struct {
	mu    sync.Mutex
	codes map[string]string
}

func NewFakeOTPSender() *FakeOTPSender { return &FakeOTPSender{codes: map[string]string{}} }

func (f *FakeOTPSender) Send(_ context.Context, phone, code string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.codes[phone] = code
	return nil
}

// LastCode returns the newest code sent to the E.164 phone.
func (f *FakeOTPSender) LastCode(phone string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.codes[phone]
	return c, ok
}
