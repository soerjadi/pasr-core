package domain

import "testing"

func TestSafeAdURL(t *testing.T) {
	for raw, want := range map[string]bool{
		"https://bri.co.id":       true,
		"https://example.id/a?b=": true,
		"http://bri.co.id":        false,
		"https://":                false,
		"javascript:alert(1)":     false,
		"":                        false,
		"//bri.co.id":             false,
		"HTTPS://bri.co.id":       true,
	} {
		if got := SafeAdURL(raw); got != want {
			t.Errorf("SafeAdURL(%q) = %v, want %v", raw, got, want)
		}
	}
}
