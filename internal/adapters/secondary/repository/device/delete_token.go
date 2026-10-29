package device

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *deviceRepository) DeleteToken(ctx context.Context, fcmToken string) error {
	ctx, span := tracer.Start(ctx, "repository.device.DeleteToken")
	defer span.End()

	if _, err := r.db.Exec(ctx, deleteTokenQuery, fcmToken); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("device.DeleteToken exec: %w", err)
	}
	return nil
}
