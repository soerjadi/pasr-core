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

	// ListCampaigns pages every campaign newest first (id DESC); beforeID > 0 keeps only smaller ids.
	ListCampaigns(ctx context.Context, beforeID int64, limit int) ([]domain.AdCampaign, error)
	// LockCampaign reads a campaign FOR UPDATE (inside a transaction), or returns nil, nil.
	LockCampaign(ctx context.Context, id int64) (*domain.AdCampaign, error)
	// CreateCampaign inserts an already validated campaign and returns the stored row.
	CreateCampaign(ctx context.Context, c domain.AdCampaign) (domain.AdCampaign, error)
	// UpdateCampaign overwrites every field of campaign c.ID, or fails with domain.ErrNotFound.
	UpdateCampaign(ctx context.Context, c domain.AdCampaign) (domain.AdCampaign, error)
	// DeleteCampaign reports whether a campaign was deleted.
	DeleteCampaign(ctx context.Context, id int64) (bool, error)
}
