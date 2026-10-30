package idempotency

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

// claimAttempts bounds the retry for the rare case where the conflicting row vanished between the
// insert and the read (its order or user was deleted).
const claimAttempts = 3

func (r *idempotencyRepository) Claim(ctx context.Context, userID int64, key, requestHash string) (int64, bool, string, error) {
	ctx, span := tracer.Start(ctx, "repository.idempotency.Claim")
	defer span.End()

	for range claimAttempts {
		tag, err := r.db.Exec(ctx, claimQuery, userID, key, requestHash)
		if err != nil {
			tracer.RecordError(span, err)
			return 0, false, "", fmt.Errorf("idempotency.Claim insert: %w", err)
		}
		if tag.RowsAffected() == 1 {
			return 0, true, "", nil
		}
		var orderID int64
		var hash string
		err = r.db.QueryRow(ctx, getQuery, userID, key).Scan(&orderID, &hash)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			tracer.RecordError(span, err)
			return 0, false, "", fmt.Errorf("idempotency.Claim select: %w", err)
		}
		return orderID, false, hash, nil
	}
	err := fmt.Errorf("idempotency.Claim: key kept disappearing")
	tracer.RecordError(span, err)
	return 0, false, "", err
}
