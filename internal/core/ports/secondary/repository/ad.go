package repository

import (
	"context"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type AdRepository interface {
	// ListActive returns the placement's active campaigns whose window contains now
	// (starts_at <= now < ends_at, null bounds open), ordered priority DESC, id.
	ListActive(ctx context.Context, placement string, now time.Time) ([]domain.Ad, error)
}
