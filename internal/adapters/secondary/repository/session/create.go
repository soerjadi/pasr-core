package session

import (
	"context"
	"fmt"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *sessionRepository) Create(ctx context.Context, t domain.RefreshToken, now time.Time) error {
	ctx, span := tracer.Start(ctx, "repository.session.Create")
	defer span.End()

	if _, err := r.db.Exec(ctx, createQuery, t.UserID, t.FamilyID, t.TokenHash, t.ExpiresAt, now); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("session.Create exec: %w", err)
	}
	return nil
}
