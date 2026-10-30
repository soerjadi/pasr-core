package rest

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

func TestWriteServiceError_CodedError(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{"conflict", domain.NewError(domain.ErrConflict, "STORE_EXISTS", "x"), 409, "STORE_EXISTS", "x"},
		{"wrapped", fmt.Errorf("wrap: %w", domain.NewError(domain.ErrConflict, "STORE_EXISTS", "x")), 409, "STORE_EXISTS", "x"},
		{"unprocessable", domain.NewError(domain.ErrUnprocessable, "IDEMPOTENCY_KEY_REUSED", "y"), 422, "IDEMPOTENCY_KEY_REUSED", "y"},
		{"too many", domain.NewError(domain.ErrTooManyRequests, "OTP_RATE_LIMITED", "z"), 429, "OTP_RATE_LIMITED", "z"},
		{"not found", domain.NewError(domain.ErrNotFound, "ORDER_NOT_FOUND", "n"), 404, "ORDER_NOT_FOUND", "n"},
		{"plain conflict", domain.ErrConflict, 409, "CONFLICT", "conflict"},
		{"plain not found", domain.ErrNotFound, 404, "NOT_FOUND", "not found"},
		{"plain unprocessable", domain.ErrUnprocessable, 422, "UNPROCESSABLE", "unprocessable"},
		{"plain too many", domain.ErrTooManyRequests, 429, "TOO_MANY_REQUESTS", "too many requests"},
		{"statement timeout", fmt.Errorf("repo: %w", &pgconn.PgError{Code: "57014"}), 503, "SERVICE_UNAVAILABLE", "service temporarily unavailable, retry later"},
		{"NUL in text", fmt.Errorf("repo: %w", &pgconn.PgError{Code: "22021"}), 400, "VALIDATION_FAILED", "text must not contain NUL characters"},
		{"other pg error", &pgconn.PgError{Code: "23505"}, 500, "INTERNAL", "internal server error"},
		{"other", errors.New("boom"), 500, "INTERNAL", "internal server error"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			WriteServiceError(rec, httptest.NewRequest(http.MethodGet, "/", nil), tc.err)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			var got ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Code != tc.code || got.Message != tc.message {
				t.Fatalf("got %+v, want code=%s message=%s", got, tc.code, tc.message)
			}
		})
	}
}

func TestWriteServiceError_StatementTimeoutIsRetryable(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteServiceError(rec, httptest.NewRequest(http.MethodGet, "/", nil), &pgconn.PgError{Code: "57014"})
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("status %d Retry-After %q", rec.Code, rec.Header().Get("Retry-After"))
	}
}
