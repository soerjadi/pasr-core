package repository

import (
	"context"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type SessionRepository interface {
	Create(ctx context.Context, t domain.RefreshToken, now time.Time) error
	// GetByHash returns domain.ErrNotFound for an unknown token.
	GetByHash(ctx context.Context, tokenHash string) (domain.RefreshToken, error)
	// Revoke reports false when the token was already revoked.
	Revoke(ctx context.Context, id int64, now time.Time) (bool, error)
	RevokeFamily(ctx context.Context, familyID string, now time.Time) error
}
