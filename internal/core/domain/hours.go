package domain

import (
	"fmt"
	"strings"
)

// DayHours is one weekday of a store's opening hours. Open and Close are "HH:MM" and unused when Closed.
type DayHours struct {
	Day    string `json:"day"`
	Closed bool   `json:"closed"`
	Open   string `json:"open,omitempty"`
	Close  string `json:"close,omitempty"`
}

// Hours holds seven entries, Monday to Sunday. A nil Hours means the seller has not set them yet.
type Hours []DayHours

var dayLabels = [7]string{"Sen", "Sel", "Rab", "Kam", "Jum", "Sab", "Min"}

// Validate checks there are seven days and that every open day has HH:MM times with open before close.
func (h Hours) Validate() error {
	if len(h) != 7 {
		return fmt.Errorf("hours must have 7 entries, got %d: %w", len(h), ErrValidationFailed)
	}
	for i, d := range h {
		if d.Closed {
			continue
		}
		open, ok1 := parseClock(d.Open)
		closeAt, ok2 := parseClock(d.Close)
		if !ok1 || !ok2 {
			return fmt.Errorf("%s: times must be HH:MM: %w", dayLabels[i], ErrValidationFailed)
		}
		if open >= closeAt {
			return fmt.Errorf("%s: open must be before close: %w", dayLabels[i], ErrValidationFailed)
		}
	}
	return nil
}

// parseClock returns minutes since midnight for a strict HH:MM string.
func parseClock(s string) (int, bool) {
	if len(s) != 5 || s[2] != ':' {
		return 0, false
	}
	for _, i := range []int{0, 1, 3, 4} {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	hh := int(s[0]-'0')*10 + int(s[1]-'0')
	mm := int(s[3]-'0')*10 + int(s[4]-'0')
	if hh > 23 || mm > 59 {
		return 0, false
	}
	return hh*60 + mm, true
}

// Display renders the hours for buyers, grouping consecutive days with identical hours,
// for example "Sen–Jum 07.00–15.00, Sab 07.00–12.00, Min tutup".
func (h Hours) Display() string {
	if len(h) != 7 {
		return "Belum diatur"
	}
	text := func(d DayHours) string {
		if d.Closed {
			return "tutup"
		}
		return strings.ReplaceAll(d.Open, ":", ".") + "–" + strings.ReplaceAll(d.Close, ":", ".")
	}
	var parts []string
	for i := 0; i < 7; {
		j := i
		for j+1 < 7 && text(h[j+1]) == text(h[i]) {
			j++
		}
		label := dayLabels[i]
		if j > i {
			label += "–" + dayLabels[j]
		}
		parts = append(parts, label+" "+text(h[i]))
		i = j + 1
	}
	if len(parts) == 1 {
		return "Setiap hari " + text(h[0])
	}
	return strings.Join(parts, ", ")
}
