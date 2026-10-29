package domain

import (
	"errors"
	"testing"
)

func TestNormalizePhone(t *testing.T) {
	ok := map[string]string{
		"0812-3456-7890":    "+6281234567890",
		"+62 812 3456 7890": "+6281234567890",
		"6281234567890":     "+6281234567890",
		"(0812) 3456 7890":  "+6281234567890",
		"  081234567890  ":  "+6281234567890",
		"0812345678":        "+62812345678",    // 9-digit national number
		"0812345678901":     "+62812345678901", // 12-digit national number
	}
	for in, want := range ok {
		got, err := NormalizePhone(in)
		if err != nil || got != want {
			t.Errorf("NormalizePhone(%q) = %q, %v; want %q", in, got, err, want)
		}
	}

	bad := []string{
		"021123456",       // landline
		"081",             // too short
		"abc",             //
		"+6581234567",     // not Indonesia
		"08123456789012",  // 13-digit national number
		"081234567",       // 8-digit national number
		"",                //
		"+62",             //
		"812345678",       // no prefix
		"08123x56789",     // letter inside
		"++6281234567890", //
		"+62 0812345678",  // national number starts with 0
	}
	for _, in := range bad {
		got, err := NormalizePhone(in)
		if !errors.Is(err, ErrValidationFailed) || got != "" {
			t.Errorf("NormalizePhone(%q) = %q, %v; want ErrValidationFailed", in, got, err)
		}
	}
}
