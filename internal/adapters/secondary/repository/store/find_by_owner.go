package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *storeRepository) FindByOwner(ctx context.Context, ownerID int64) (*domain.MyStoreRecord, error) {
	ctx, span := tracer.Start(ctx, "repository.store.FindByOwner")
	defer span.End()

	var (
		s      domain.MyStoreRecord
		status string
	)
	err := r.db.QueryRow(ctx, findByOwnerQuery, ownerID).Scan(
		&s.ID, &s.Name, &s.CategoryID, &s.OwnerName, &s.Kelurahan, &s.Address, &s.Description,
		&s.PhotoUploadID, &s.PhotoKeys, &status, &s.IsOpen, &s.WhatsApp, &s.Hours,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("store.FindByOwner query: %w", err)
	}
	s.Status = domain.StoreStatus(status)
	return &s, nil
}
