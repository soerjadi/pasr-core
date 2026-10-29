package notify

import (
	"time"

	"github.com/oemah/nusantara/core/internal/core/ports/services"
)

// Tune overrides the batch size and per-send timeout of a service built by NewService.
func Tune(s services.NotifyService, batch int, sendTimeout time.Duration) {
	svc := s.(*service)
	if batch > 0 {
		svc.batch = batch
	}
	if sendTimeout > 0 {
		svc.sendTimeout = sendTimeout
	}
}
