package domain

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"
)

func TestTransition_Table(t *testing.T) {
	all := []OrderStatus{OrderCreated, OrderSellerProcessing, OrderReadyForPickup, OrderCompleted, OrderCancelled}
	actions := []Action{ActionAccept, ActionReject, ActionReady, ActionComplete, ActionCancel, ActionExpire}
	type key struct {
		from OrderStatus
		a    Action
	}
	legal := map[key]OrderStatus{
		{OrderCreated, ActionAccept}:          OrderSellerProcessing,
		{OrderCreated, ActionReject}:          OrderCancelled,
		{OrderSellerProcessing, ActionReady}:  OrderReadyForPickup,
		{OrderReadyForPickup, ActionComplete}: OrderCompleted,
		{OrderSellerProcessing, ActionCancel}: OrderCancelled,
		{OrderReadyForPickup, ActionCancel}:   OrderCancelled,
		{OrderCreated, ActionExpire}:          OrderCancelled,
	}
	for _, role := range []string{"seller", "operator", "system", "buyer"} {
		actor := Actor{Role: role, UserID: 1}
		for _, from := range all {
			for _, a := range actions {
				want, ok := legal[key{from, a}]
				// Role gating: system only expires; buyer does nothing.
				switch {
				case role == "buyer":
					ok = false
				case a == ActionExpire && role != "system":
					ok = false
				case role == "system" && a != ActionExpire:
					ok = false
				}
				got, err := Transition(from, a, actor)
				if ok {
					if err != nil || got != want {
						t.Errorf("%s %s %s: got (%s, %v), want %s", role, from, a, got, err, want)
					}
					continue
				}
				var ce *CodedError
				if !errors.As(err, &ce) || !errors.Is(err, ErrConflict) || ce.Code != "ORDER_NOT_ACTIONABLE" {
					t.Errorf("%s %s %s: want ORDER_NOT_ACTIONABLE conflict, got %v", role, from, a, err)
				}
				if got != from {
					t.Errorf("%s %s %s: status on error = %s, want unchanged", role, from, a, got)
				}
			}
		}
	}
}

func TestTransition_ExpireOnlyBySystem(t *testing.T) {
	for _, role := range []string{"seller", "operator", "buyer"} {
		if _, err := Transition(OrderCreated, ActionExpire, Actor{Role: role}); err == nil {
			t.Errorf("%s must not expire", role)
		}
	}
	got, err := Transition(OrderCreated, ActionExpire, Actor{Role: "system"})
	if err != nil || got != OrderCancelled {
		t.Fatalf("system expire = (%s, %v)", got, err)
	}
}

func TestDeriveOrderStatus_Table(t *testing.T) {
	tests := []struct {
		name string
		in   []OrderStatus
		want OrderStatus
	}{
		{"single", []OrderStatus{OrderReadyForPickup}, OrderReadyForPickup},
		{"mixed", []OrderStatus{OrderCompleted, OrderSellerProcessing, OrderReadyForPickup}, OrderSellerProcessing},
		{"created wins", []OrderStatus{OrderCompleted, OrderCreated}, OrderCreated},
		{"one cancelled", []OrderStatus{OrderCancelled, OrderReadyForPickup, OrderCompleted}, OrderReadyForPickup},
		{"all cancelled", []OrderStatus{OrderCancelled, OrderCancelled}, OrderCancelled},
		{"all completed", []OrderStatus{OrderCompleted, OrderCompleted}, OrderCompleted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DeriveOrderStatus(tt.in); got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestDeriveOrderStatus_EmptyPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	DeriveOrderStatus(nil)
}

func TestTabStatuses(t *testing.T) {
	tests := []struct {
		tab     string
		want    []OrderStatus
		wantErr bool
	}{
		{"ALL", nil, false},
		{"PROCESSING", []OrderStatus{OrderCreated, OrderSellerProcessing}, false},
		{"READY", []OrderStatus{OrderReadyForPickup}, false},
		{"DONE", []OrderStatus{OrderCompleted, OrderCancelled}, false},
		{"bogus", nil, true},
		{"", nil, true},
	}
	for _, tt := range tests {
		got, err := TabStatuses(tt.tab)
		if (err != nil) != tt.wantErr || (err != nil && !errors.Is(err, ErrValidationFailed)) {
			t.Errorf("%q: err = %v", tt.tab, err)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q: got %v, want %v", tt.tab, got, tt.want)
		}
	}
}

func TestSellerTabStatuses(t *testing.T) {
	tests := []struct {
		tab     string
		want    []OrderStatus
		wantErr bool
	}{
		{"NEW", []OrderStatus{OrderCreated}, false},
		{"PROCESSING", []OrderStatus{OrderSellerProcessing, OrderReadyForPickup}, false},
		{"DONE", []OrderStatus{OrderCompleted, OrderCancelled}, false},
		{"ALL", nil, true},
	}
	for _, tt := range tests {
		got, err := SellerTabStatuses(tt.tab)
		if (err != nil) != tt.wantErr || (err != nil && !errors.Is(err, ErrValidationFailed)) {
			t.Errorf("%q: err = %v", tt.tab, err)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q: got %v, want %v", tt.tab, got, tt.want)
		}
	}
}

func TestLineTotal_Overflow(t *testing.T) {
	if got, err := LineTotal(15000, 3); err != nil || got != 45000 {
		t.Fatalf("got (%d, %v)", got, err)
	}
	if got, err := LineTotal(0, 5); err != nil || got != 0 {
		t.Fatalf("zero price: (%d, %v)", got, err)
	}
	_, err := LineTotal(math.MaxInt64/2+1, 2)
	if err == nil || !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("want validation error on overflow, got %v", err)
	}
	if _, err := LineTotal(100, 0); err == nil {
		t.Fatal("qty 0 must fail")
	}
	if _, err := LineTotal(-1, 1); err == nil {
		t.Fatal("negative price must fail")
	}
}

func TestFormatOrderNumber_WIBDateBoundary(t *testing.T) {
	wib := time.FixedZone("WIB", 7*3600)
	utc := time.Date(2026, 10, 14, 23, 30, 0, 0, time.UTC) // 06:30 on the 15th in WIB
	if got := FormatOrderNumber(utc.In(wib), 1); got != "NM-20261015-001" {
		t.Errorf("got %s", got)
	}
	if got := FormatOrderNumber(utc, 1); got != "NM-20261014-001" {
		t.Errorf("UTC date: got %s", got)
	}
	if got := FormatOrderNumber(utc.In(wib), 1234); got != "NM-20261015-1234" {
		t.Errorf("wide seq: got %s", got)
	}
}
