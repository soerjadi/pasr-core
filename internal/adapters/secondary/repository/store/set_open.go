package store

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *storeRepository) SetOpen(ctx context.Context, storeID int64, open bool) error {
	ctx, span := tracer.Start(ctx, "repository.store.SetOpen")
	defer span.End()

	if _, err := r.db.Exec(ctx, setOpenQuery, storeID, open); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("store.SetOpen exec: %w", err)
	}
	return nil
}
