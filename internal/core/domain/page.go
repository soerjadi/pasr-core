package domain

import (
	"encoding/base64"
	"fmt"
	"strconv"
)

const (
	defaultLimit = 20
	maxLimit     = 50
)

// Page is the list envelope; NextCursor is nil on the last page.
type Page[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"nextCursor"`
}

func EncodeCursor(id int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(id, 10)))
}

// DecodeCursor returns 0 for an empty cursor and ErrValidationFailed for a malformed one.
func DecodeCursor(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return 0, fmt.Errorf("invalid cursor: %w", ErrValidationFailed)
	}
	id, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid cursor: %w", ErrValidationFailed)
	}
	return id, nil
}

func ClampLimit(n int) int {
	if n <= 0 {
		return defaultLimit
	}
	if n > maxLimit {
		return maxLimit
	}
	return n
}

// BuildPage expects limit+1 rows fetched; it trims to limit and sets NextCursor from the last kept row.
func BuildPage[T any](rows []T, limit int, id func(T) int64) Page[T] {
	if len(rows) > limit {
		rows = rows[:limit]
		c := EncodeCursor(id(rows[limit-1]))
		return Page[T]{Items: rows, NextCursor: &c}
	}
	if rows == nil {
		rows = []T{}
	}
	return Page[T]{Items: rows}
}
