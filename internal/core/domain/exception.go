package domain

import "errors"

var (
	ErrNotFound         = errors.New("not found")
	ErrValidationFailed = errors.New("validation failed")
	ErrConflict         = errors.New("conflict")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrForbidden        = errors.New("forbidden")
)
