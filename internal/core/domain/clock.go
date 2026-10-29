package domain

import "time"

// Clock lets services and tests control time.
type Clock interface {
	Now() time.Time
}

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }
