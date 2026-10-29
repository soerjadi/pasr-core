package otp

import "context"

// Sender delivers a one-time code to a phone number (E.164).
type Sender interface {
	Send(ctx context.Context, phone, code string) error
}
