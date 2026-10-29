package testapp

import (
	"context"
	"sync"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

// PushSend is one recorded push attempt.
type PushSend struct {
	Token        string
	Notification domain.Notification
}

// FakePusher records every send and is safe for concurrent use.
type FakePusher struct {
	mu           sync.Mutex
	sends        []PushSend
	failWith     error
	unregistered map[string]bool
	delay        func()
}

func NewFakePusher() *FakePusher { return &FakePusher{unregistered: map[string]bool{}} }

// FailWith makes every following Send return err (nil restores success).
func (p *FakePusher) FailWith(err error) { p.mu.Lock(); p.failWith = err; p.mu.Unlock() }

// MarkUnregistered makes Send report the token as unregistered.
func (p *FakePusher) MarkUnregistered(token string) {
	p.mu.Lock()
	p.unregistered[token] = true
	p.mu.Unlock()
}

// SetDelay runs fn inside every Send, before it records; tests use it to widen race windows.
func (p *FakePusher) SetDelay(fn func()) { p.mu.Lock(); p.delay = fn; p.mu.Unlock() }

// Sends returns a copy of the recorded sends in order.
func (p *FakePusher) Sends() []PushSend {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]PushSend(nil), p.sends...)
}

func (p *FakePusher) Send(_ context.Context, token string, n domain.Notification) (bool, error) {
	p.mu.Lock()
	delay := p.delay
	p.mu.Unlock()
	if delay != nil {
		delay()
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sends = append(p.sends, PushSend{Token: token, Notification: n})
	if p.failWith != nil {
		return false, p.failWith
	}
	return p.unregistered[token], nil
}
