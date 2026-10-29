package domain

import (
	"errors"
	"testing"
)

func week(open, closeAt string) Hours {
	names := []string{"MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"}
	h := make(Hours, 7)
	for i, n := range names {
		h[i] = DayHours{Day: n, Open: open, Close: closeAt}
	}
	return h
}

func TestHours_Display(t *testing.T) {
	split := week("07:00", "15:00")
	split[5].Close = "12:00"
	split[6] = DayHours{Day: "SUN", Closed: true}

	midClosed := week("08:00", "16:00")
	midClosed[2] = DayHours{Day: "WED", Closed: true}

	tests := []struct {
		name string
		h    Hours
		want string
	}{
		{"nil", nil, "Belum diatur"},
		{"all same", week("07:00", "17:00"), "Setiap hari 07.00–17.00"},
		{"split weekend with closed sunday", split, "Sen–Jum 07.00–15.00, Sab 07.00–12.00, Min tutup"},
		{"closed day in the middle", midClosed, "Sen–Sel 08.00–16.00, Rab tutup, Kam–Min 08.00–16.00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.h.Display(); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHours_ValidateRejectsCloseBeforeOpen(t *testing.T) {
	bad := week("07:00", "17:00")
	bad[3] = DayHours{Day: "THU", Open: "17:00", Close: "07:00"}
	if err := bad.Validate(); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("close before open: got %v", err)
	}
	equal := week("07:00", "07:00")
	if err := equal.Validate(); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("open == close: got %v", err)
	}
	for _, s := range []string{"7:00", "25:00", "07:60", "ab:cd", ""} {
		if err := week(s, "17:00").Validate(); !errors.Is(err, ErrValidationFailed) {
			t.Fatalf("time %q: got %v", s, err)
		}
	}
	if err := (Hours{}).Validate(); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("wrong length: got %v", err)
	}
	ok := week("07:00", "17:00")
	ok[6] = DayHours{Day: "SUN", Closed: true}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid hours: %v", err)
	}
}
