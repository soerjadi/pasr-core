package fcm

import "time"

// DefaultTimeout exposes the timeout of the client New builds when none is injected.
func DefaultTimeout() time.Duration {
	return New(Config{ProjectID: "x"}).(*pusher).client.Timeout
}
