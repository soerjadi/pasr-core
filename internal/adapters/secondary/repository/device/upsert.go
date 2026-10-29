package device

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *deviceRepository) Upsert(ctx context.Context, userID int64, fcmToken string) error {
	ctx, span := tracer.Start(ctx, "repository.device.Upsert")
	defer span.End()

	if _, err := r.db.Exec(ctx, upsertQuery, userID, fcmToken); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("device.Upsert exec: %w", err)
	}
	return nil
}
