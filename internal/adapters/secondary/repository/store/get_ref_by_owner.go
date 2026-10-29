package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *storeRepository) GetRefByOwner(ctx context.Context, ownerID int64) (*domain.StoreRef, error) {
	ctx, span := tracer.Start(ctx, "repository.store.GetRefByOwner")
	defer span.End()

	var ref domain.StoreRef
	err := r.db.QueryRow(ctx, getRefByOwnerQuery, ownerID).Scan(&ref.ID, &ref.CategoryID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("store.GetRefByOwner query: %w", err)
	}
	return &ref, nil
}
