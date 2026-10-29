package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *storeRepository) GetSummaryByOwner(ctx context.Context, ownerID int64) (*domain.StoreSummary, error) {
	ctx, span := tracer.Start(ctx, "repository.store.GetSummaryByOwner")
	defer span.End()

	var (
		s        domain.StoreSummary
		earliest *time.Time
	)
	err := r.db.QueryRow(ctx, getSummaryByOwnerQuery, ownerID).Scan(&s.ID, &s.Name, &s.Status, &s.NewOrderCount, &earliest)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("store.GetSummaryByOwner query: %w", err)
	}
	if earliest != nil {
		u := earliest.UTC()
		s.AcceptBy = &u
	}
	return &s, nil
}
