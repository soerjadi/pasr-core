package services

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type AdsService interface {
	// List returns the placement's live ads, highest priority first. An unknown or empty
	// placement fails with domain.ErrValidationFailed. Campaigns with a non-https target or a
	// blank label are skipped.
	List(ctx context.Context, placement string) ([]domain.Ad, error)
}
