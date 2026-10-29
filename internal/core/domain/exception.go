package domain

import "errors"

var (
	ErrNotFound         = errors.New("not found")
	ErrValidationFailed = errors.New("validation failed")
	ErrConflict         = errors.New("conflict")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrForbidden        = errors.New("forbidden")
	ErrUnprocessable    = errors.New("unprocessable")
	ErrTooManyRequests  = errors.New("too many requests")
)

// CodedError carries a stable API error code and message on top of a kind sentinel.
// errors.Is(err, Kind) holds, so the kind decides the HTTP status.
type CodedError struct {
	Kind    error
	Code    string
	Message string
}

func (e *CodedError) Error() string { return e.Message }

func (e *CodedError) Unwrap() error { return e.Kind }

// NewError builds a CodedError of the given kind.
func NewError(kind error, code, message string) error {
	return &CodedError{Kind: kind, Code: code, Message: message}
}
