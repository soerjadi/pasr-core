package domain

import "strings"

const (
	minNationalDigits = 9
	maxNationalDigits = 12
)

// NormalizePhone converts an Indonesian mobile number to E.164 (+62...). It accepts 08..., 628... and +628...,
// with spaces, dashes, dots and parentheses as separators. Anything else is ErrValidationFailed.
func NormalizePhone(s string) (string, error) {
	var b strings.Builder
	for i, r := range strings.TrimSpace(s) {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '+' && i == 0:
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '.' || r == '(' || r == ')':
		default:
			return "", errInvalidPhone()
		}
	}
	p := b.String()

	var national string
	switch {
	case strings.HasPrefix(p, "+62"):
		national = p[3:]
	case strings.HasPrefix(p, "62"):
		national = p[2:]
	case strings.HasPrefix(p, "0"):
		national = p[1:]
	default:
		return "", errInvalidPhone()
	}
	if strings.ContainsRune(national, '+') || national == "" || national[0] != '8' ||
		len(national) < minNationalDigits || len(national) > maxNationalDigits {
		return "", errInvalidPhone()
	}
	return "+62" + national, nil
}

func errInvalidPhone() error {
	return NewError(ErrValidationFailed, "VALIDATION_FAILED", "invalid phone number")
}
