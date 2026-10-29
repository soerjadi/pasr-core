// Package testapp builds the real application on a throwaway PostgreSQL behind an httptest server.
package testapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"
	"time"

	"github.com/oemah/nusantara/core/internal/app"
	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/config"
	"github.com/oemah/nusantara/core/internal/testutil/pgtest"
)

// Env is a running application under test.
type Env struct {
	t        *testing.T
	Server   *httptest.Server
	Clock    *FakeClock     // the application clock; advance it to test expiry
	OTP      *FakeOTPSender // records the latest code sent per phone
	Storage  *MemStorage    // in-memory object storage injected into the app
	Push     *FakePusher    // records every push the dispatcher sends
	opPhones map[string]bool
}

// Test secrets and the phones that receive the operator role.
const (
	AccessSecret     = "test-access-secret"
	RefreshSecret    = "test-refresh-secret"
	OTPSecret        = "test-otp-secret"
	OperatorPhone    = "+6281999000001"
	OperatorPhone2   = "+6281999000002"
	defaultAccessTTL = 15
)

// Option customises the application under test.
type Option func(*config.Config)

// WithConfig mutates the test config before the app is built.
func WithConfig(fn func(*config.Config)) Option { return fn }

// Resp is a recorded HTTP response.
type Resp struct {
	Status int
	Header http.Header
	Body   []byte
	t      *testing.T
}

// Opt customises a request.
type Opt func(*http.Request)

// Bearer sets the Authorization header.
func Bearer(token string) Opt {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token) }
}

// Header sets an arbitrary request header.
func Header(k, v string) Opt {
	return func(r *http.Request) { r.Header.Set(k, v) }
}

// New starts the real app against the shared test database. Skips under -short.
func New(t *testing.T, opts ...Option) *Env {
	t.Helper()
	url := pgtest.URL(t)
	clock := NewFakeClock(time.Now())
	sender := NewFakeOTPSender()
	store := NewMemStorage()
	pusher := NewFakePusher()

	cfg := &config.Config{
		App:      config.App{Port: 0, Env: "test", ShutdownTimeout: 1},
		Database: config.Database{URL: url, MaxConns: 5, MinConns: 0, MaxConnLifetime: 30, MaxConnIdleTime: 5},
		JWT:      config.JWT{AccessSecret: AccessSecret, RefreshSecret: RefreshSecret, AccessExpiry: defaultAccessTTL, RefreshExpiry: 60},
		OTP: config.OTP{
			Secret: OTPSecret, TTLSeconds: 300, MaxAttempts: 5, PhoneLimit: 100, IPLimit: 1000, WindowMinutes: 10, Sender: "log",
		},
		Images: config.Images{MaxBytes: 1 << 20, MaxPixels: 4_000_000},
	}
	cfg.App.OperatorPhones = []string{OperatorPhone, OperatorPhone2}
	for _, o := range opts {
		o(cfg)
	}
	a, err := app.New(context.Background(), cfg, app.Options{Clock: clock, OTPSender: sender, Storage: store, Pusher: pusher})
	if err != nil {
		t.Fatalf("testapp: build app: %v", err)
	}
	srv := httptest.NewServer(a.Router())
	t.Cleanup(func() {
		srv.Close()
		_ = a.Shutdown(context.Background())
	})
	ops := map[string]bool{}
	for _, p := range cfg.App.OperatorPhones {
		if n, err := domain.NormalizePhone(p); err == nil {
			ops[n] = true
		}
	}
	return &Env{t: t, Server: srv, Clock: clock, OTP: sender, Storage: store, Push: pusher, opPhones: ops}
}

// Do sends a request; a non-nil body is JSON-encoded.
func (e *Env) Do(method, path string, body any, opts ...Opt) *Resp {
	e.t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			e.t.Fatalf("testapp: marshal body: %v", err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, e.Server.URL+path, rd)
	if err != nil {
		e.t.Fatalf("testapp: new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, o := range opts {
		o(req)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatalf("testapp: %s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		e.t.Fatalf("testapp: read body: %v", err)
	}
	return &Resp{Status: res.StatusCode, Header: res.Header, Body: b, t: e.t}
}

// JSON decodes the response body into into.
func (r *Resp) JSON(into any) {
	r.t.Helper()
	if err := json.Unmarshal(r.Body, into); err != nil {
		r.t.Fatalf("testapp: decode %s: %v", r.Body, err)
	}
}

// SignIn logs the phone in through the real OTP flow and returns the session.
func (e *Env) SignIn(phone string) domain.Session {
	e.t.Helper()
	norm, err := domain.NormalizePhone(phone)
	if err != nil {
		e.t.Fatalf("testapp: SignIn phone %q: %v", phone, err)
	}
	if r := e.Do(http.MethodPost, "/v1/auth/otp/request", map[string]string{"phone": norm}); r.Status != http.StatusNoContent {
		e.t.Fatalf("testapp: request otp: status %d body %s", r.Status, r.Body)
	}
	code, ok := e.OTP.LastCode(norm)
	if !ok {
		e.t.Fatalf("testapp: no code was sent to %s", norm)
	}
	r := e.Do(http.MethodPost, "/v1/auth/otp/verify", map[string]string{"phone": norm, "code": code})
	if r.Status != http.StatusOK {
		e.t.Fatalf("testapp: verify otp: status %d body %s", r.Status, r.Body)
	}
	var s domain.Session
	r.JSON(&s)
	return s
}

// SignInOperator signs in a phone configured as an operator (OperatorPhone or OperatorPhone2 by default).
func (e *Env) SignInOperator(phone string) domain.Session {
	e.t.Helper()
	norm, err := domain.NormalizePhone(phone)
	if err != nil || !e.opPhones[norm] {
		e.t.Fatalf("testapp: SignInOperator: %q is not a configured operator phone (use OperatorPhone or set App.OperatorPhones via WithConfig)", phone)
	}
	return e.SignIn(phone)
}

// DoFile sends a multipart/form-data request with one file part.
func (e *Env) DoFile(method, path, field, filename, contentType string, data []byte, opts ...Opt) *Resp {
	e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, field, filename))
	h.Set("Content-Type", contentType)
	part, err := mw.CreatePart(h)
	if err != nil {
		e.t.Fatalf("testapp: multipart part: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		e.t.Fatalf("testapp: multipart write: %v", err)
	}
	if err := mw.Close(); err != nil {
		e.t.Fatalf("testapp: multipart close: %v", err)
	}
	req, err := http.NewRequest(method, e.Server.URL+path, &buf)
	if err != nil {
		e.t.Fatalf("testapp: new request: %v", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	for _, o := range opts {
		o(req)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatalf("testapp: %s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		e.t.Fatalf("testapp: read body: %v", err)
	}
	return &Resp{Status: res.StatusCode, Header: res.Header, Body: b, t: e.t}
}
