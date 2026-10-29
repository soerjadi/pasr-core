package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *sessionRepository) GetByHash(ctx context.Context, tokenHash string) (domain.RefreshToken, error) {
	ctx, span := tracer.Start(ctx, "repository.session.GetByHash")
	defer span.End()

	var t domain.RefreshToken
	err := r.db.QueryRow(ctx, getByHashQuery, tokenHash).
		Scan(&t.ID, &t.UserID, &t.FamilyID, &t.TokenHash, &t.ExpiresAt, &t.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RefreshToken{}, domain.ErrNotFound
	}
	if err != nil {
		tracer.RecordError(span, err)
		return domain.RefreshToken{}, fmt.Errorf("session.GetByHash query: %w", err)
	}
	return t, nil
}
