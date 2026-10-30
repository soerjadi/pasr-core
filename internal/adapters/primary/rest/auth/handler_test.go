package auth_test

import (
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/services/identity"
	"github.com/oemah/nusantara/core/internal/infrastructure/config"
	"github.com/oemah/nusantara/core/internal/testutil/contract"
	"github.com/oemah/nusantara/core/internal/testutil/testapp"
)

const phone = "+6281234567890"

type errBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func requestOTP(env *testapp.Env, ph string, opts ...testapp.Opt) *testapp.Resp {
	return env.Do(http.MethodPost, "/v1/auth/otp/request", map[string]string{"phone": ph}, opts...)
}

func verify(env *testapp.Env, ph, code string) *testapp.Resp {
	return env.Do(http.MethodPost, "/v1/auth/otp/verify", map[string]string{"phone": ph, "code": code})
}

func refresh(env *testapp.Env, tok string) *testapp.Resp {
	return env.Do(http.MethodPost, "/v1/auth/refresh", map[string]string{"refreshToken": tok})
}

func wrongCode(real string) string {
	if real == "000000" {
		return "000001"
	}
	return "000000"
}

func assertErr(t *testing.T, r *testapp.Resp, status int, code string) {
	t.Helper()
	var e errBody
	r.JSON(&e)
	if r.Status != status || e.Code != code {
		t.Fatalf("want %d %s, got %d %s", status, code, r.Status, r.Body)
	}
}

func TestRequestOTP_204(t *testing.T) {
	env := testapp.New(t)
	if r := requestOTP(env, "0812-3456-7890"); r.Status != http.StatusNoContent {
		t.Fatalf("status %d body %s", r.Status, r.Body)
	}
	code, ok := env.OTP.LastCode(phone)
	if !ok || len(code) != 6 {
		t.Fatalf("code = %q, ok = %v", code, ok)
	}
	if r := requestOTP(env, "021123456"); r.Status != http.StatusBadRequest {
		t.Fatalf("invalid phone: status %d", r.Status)
	}
}

func TestRequestOTP_RateLimitedPerPhone(t *testing.T) {
	env := testapp.New(t, testapp.WithConfig(func(c *config.Config) { c.OTP.PhoneLimit = 2 }))
	for i := 0; i < 2; i++ {
		if r := requestOTP(env, phone); r.Status != http.StatusNoContent {
			t.Fatalf("request %d: status %d", i, r.Status)
		}
	}
	assertErr(t, requestOTP(env, phone), http.StatusTooManyRequests, "OTP_RATE_LIMITED")
	if r := requestOTP(env, "+6281234567891"); r.Status != http.StatusNoContent {
		t.Fatalf("other phone: status %d", r.Status)
	}
	env.Clock.Advance(11 * time.Minute)
	if r := requestOTP(env, phone); r.Status != http.StatusNoContent {
		t.Fatalf("after window: status %d", r.Status)
	}
}

func TestRequestOTP_RateLimitedPerIP(t *testing.T) {
	env := testapp.New(t, testapp.WithConfig(func(c *config.Config) { c.OTP.IPLimit = 2 }))
	ip := testapp.Header("CF-Connecting-IP", "203.0.113.9")
	for i, ph := range []string{"+6281234560001", "+6281234560002"} {
		if r := requestOTP(env, ph, ip); r.Status != http.StatusNoContent {
			t.Fatalf("request %d: status %d", i, r.Status)
		}
	}
	assertErr(t, requestOTP(env, "+6281234560003", ip), http.StatusTooManyRequests, "OTP_RATE_LIMITED")
	if r := requestOTP(env, "+6281234560003", testapp.Header("CF-Connecting-IP", "203.0.113.10")); r.Status != http.StatusNoContent {
		t.Fatalf("other ip: status %d", r.Status)
	}
}

func TestVerifyOTP_Success_CreatesUserAndSession(t *testing.T) {
	env := testapp.New(t)
	requestOTP(env, phone)
	code, _ := env.OTP.LastCode(phone)

	r := verify(env, "08123456 7890", code)
	if r.Status != http.StatusOK {
		t.Fatalf("status %d body %s", r.Status, r.Body)
	}
	contract.AssertShape(t, "verifyOtp", r.Body)
	var s domain.Session
	r.JSON(&s)
	if s.User.Phone != phone || s.User.ID == 0 || s.AccessToken == "" || s.RefreshToken == "" {
		t.Fatalf("session = %+v", s)
	}

	again := env.SignIn(phone)
	if again.User.ID != s.User.ID {
		t.Fatalf("same phone got a new user: %d vs %d", again.User.ID, s.User.ID)
	}
}

func TestVerifyOTP_WrongCode_400OTPInvalid(t *testing.T) {
	env := testapp.New(t)
	requestOTP(env, phone)
	code, _ := env.OTP.LastCode(phone)
	assertErr(t, verify(env, phone, wrongCode(code)), http.StatusBadRequest, "OTP_INVALID")
	assertErr(t, verify(env, "+6281234500000", "123456"), http.StatusBadRequest, "OTP_INVALID") // no code requested
	if r := verify(env, phone, code); r.Status != http.StatusOK {
		t.Fatalf("right code after a wrong one: status %d", r.Status)
	}
}

func TestVerifyOTP_ExpiredCode(t *testing.T) {
	env := testapp.New(t)
	requestOTP(env, phone)
	code, _ := env.OTP.LastCode(phone)
	env.Clock.Advance(301 * time.Second)
	assertErr(t, verify(env, phone, code), http.StatusBadRequest, "OTP_INVALID")
}

func TestVerifyOTP_AttemptCap_429AndCodeDead(t *testing.T) {
	env := testapp.New(t)
	requestOTP(env, phone)
	code, _ := env.OTP.LastCode(phone)
	for i := 0; i < 5; i++ {
		assertErr(t, verify(env, phone, wrongCode(code)), http.StatusBadRequest, "OTP_INVALID")
	}
	assertErr(t, verify(env, phone, code), http.StatusTooManyRequests, "OTP_RATE_LIMITED")
	assertErr(t, verify(env, phone, code), http.StatusTooManyRequests, "OTP_RATE_LIMITED")

	// A fresh request replaces the dead code.
	env.SignIn(phone)
}

func TestVerifyOTP_CodeCannotBeReused(t *testing.T) {
	env := testapp.New(t)
	requestOTP(env, phone)
	code, _ := env.OTP.LastCode(phone)
	if r := verify(env, phone, code); r.Status != http.StatusOK {
		t.Fatalf("first use: status %d", r.Status)
	}
	assertErr(t, verify(env, phone, code), http.StatusBadRequest, "OTP_INVALID")
}

func TestVerifyOTP_NewRequestInvalidatesEarlierCode(t *testing.T) {
	env := testapp.New(t)
	requestOTP(env, phone)
	first, _ := env.OTP.LastCode(phone)
	requestOTP(env, phone)
	second, _ := env.OTP.LastCode(phone)
	if first != second {
		assertErr(t, verify(env, phone, first), http.StatusBadRequest, "OTP_INVALID")
	}
	if r := verify(env, phone, second); r.Status != http.StatusOK {
		t.Fatalf("newest code: status %d", r.Status)
	}
}

func TestVerifyOTP_ConcurrentSameCode_OnlyOneSucceeds(t *testing.T) {
	env := testapp.New(t)
	requestOTP(env, phone)
	code, _ := env.OTP.LastCode(phone)

	const n = 4
	var ok, bad atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			r := verifyRaw(env.Server.URL, phone, code)
			switch r {
			case http.StatusOK:
				ok.Add(1)
			case http.StatusBadRequest:
				bad.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if ok.Load() != 1 || bad.Load() != n-1 {
		t.Fatalf("ok = %d, bad = %d; want 1 and %d", ok.Load(), bad.Load(), n-1)
	}
}

func TestVerifyOTP_OperatorPhoneGetsOperatorRole(t *testing.T) {
	env := testapp.New(t)
	op := env.SignInOperator(testapp.OperatorPhone)
	buyer := env.SignIn(phone)

	tokens := identity.NewTokens(testapp.AccessSecret, time.Hour, env.Clock)
	if c, err := tokens.Parse(op.AccessToken); err != nil || c.Role != domain.RoleOperator || c.UserID != op.User.ID {
		t.Fatalf("operator claims = %+v, %v", c, err)
	}
	if c, err := tokens.Parse(buyer.AccessToken); err != nil || c.Role != domain.RoleBuyer {
		t.Fatalf("buyer claims = %+v, %v", c, err)
	}
}

func TestRefresh_Rotates(t *testing.T) {
	env := testapp.New(t)
	s := env.SignIn(phone)

	r := refresh(env, s.RefreshToken)
	if r.Status != http.StatusOK {
		t.Fatalf("status %d body %s", r.Status, r.Body)
	}
	contract.AssertShape(t, "refreshSession", r.Body)
	var s2 domain.Session
	r.JSON(&s2)
	if s2.RefreshToken == s.RefreshToken || s2.AccessToken == "" || s2.User.ID != s.User.ID {
		t.Fatalf("not rotated: %+v", s2)
	}
	if r := refresh(env, s2.RefreshToken); r.Status != http.StatusOK {
		t.Fatalf("new token: status %d", r.Status)
	}
}

func TestRefresh_ReuseRevokesFamily(t *testing.T) {
	env := testapp.New(t)
	s := env.SignIn(phone)
	var s2 domain.Session
	refresh(env, s.RefreshToken).JSON(&s2)

	assertErr(t, refresh(env, s.RefreshToken), http.StatusUnauthorized, "UNAUTHORIZED")
	// The descendant is revoked with the family.
	assertErr(t, refresh(env, s2.RefreshToken), http.StatusUnauthorized, "UNAUTHORIZED")
	assertErr(t, refresh(env, "not-a-token"), http.StatusUnauthorized, "UNAUTHORIZED")
}

func TestRefresh_Expired401(t *testing.T) {
	env := testapp.New(t)
	s := env.SignIn(phone)
	env.Clock.Advance(61 * time.Minute)
	assertErr(t, refresh(env, s.RefreshToken), http.StatusUnauthorized, "UNAUTHORIZED")
}

func TestLogout_Idempotent204(t *testing.T) {
	env := testapp.New(t)
	s := env.SignIn(phone)
	logout := func(tok string) int {
		return env.Do(http.MethodPost, "/v1/auth/logout", map[string]string{"refreshToken": tok}).Status
	}
	if got := logout(s.RefreshToken); got != http.StatusNoContent {
		t.Fatalf("first logout: %d", got)
	}
	if got := logout(s.RefreshToken); got != http.StatusNoContent {
		t.Fatalf("second logout: %d", got)
	}
	if got := logout("unknown"); got != http.StatusNoContent {
		t.Fatalf("unknown token: %d", got)
	}
	assertErr(t, refresh(env, s.RefreshToken), http.StatusUnauthorized, "UNAUTHORIZED")
}

func fireParallel(env *testapp.Env, n int, phoneFor func(i int) string, ip string) (ok, limited, other int32) {
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			switch requestRaw(env.Server.URL, phoneFor(i), ip) {
			case http.StatusNoContent:
				atomic.AddInt32(&ok, 1)
			case http.StatusTooManyRequests:
				atomic.AddInt32(&limited, 1)
			default:
				atomic.AddInt32(&other, 1)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	return
}

func TestRequestOTP_ConcurrentRespectsPhoneLimit(t *testing.T) {
	env := testapp.New(t, testapp.WithConfig(func(c *config.Config) { c.OTP.PhoneLimit = 2 }))
	ok, limited, other := fireParallel(env, 8, func(int) string { return phone }, "")
	if ok != 2 || limited != 6 || other != 0 {
		t.Fatalf("204 = %d, 429 = %d, other = %d; want 2, 6, 0", ok, limited, other)
	}
}

func TestRequestOTP_ConcurrentRespectsIPLimit(t *testing.T) {
	env := testapp.New(t, testapp.WithConfig(func(c *config.Config) { c.OTP.IPLimit = 2 }))
	ok, limited, other := fireParallel(env, 8, func(i int) string { return "+628123456000" + string(rune('0'+i)) }, "203.0.113.7")
	if ok != 2 || limited != 6 || other != 0 {
		t.Fatalf("204 = %d, 429 = %d, other = %d; want 2, 6, 0", ok, limited, other)
	}
}

func TestLogout_EmptyOrMalformedBody204(t *testing.T) {
	env := testapp.New(t)
	for _, body := range []string{"", "not json", "{}"} {
		req, _ := http.NewRequest(http.MethodPost, env.Server.URL+"/v1/auth/logout", strings.NewReader(body))
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusNoContent {
			t.Fatalf("body %q: status %d", body, res.StatusCode)
		}
	}
}
