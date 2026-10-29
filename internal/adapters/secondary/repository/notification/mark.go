package notification

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *notificationRepository) MarkSent(ctx context.Context, id int64) error {
	ctx, span := tracer.Start(ctx, "repository.notification.MarkSent")
	defer span.End()

	if _, err := r.db.Exec(ctx, markSentQuery, id); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("notification.MarkSent exec: %w", err)
	}
	return nil
}

func (r *notificationRepository) MarkFailed(ctx context.Context, id int64, lastError string) error {
	ctx, span := tracer.Start(ctx, "repository.notification.MarkFailed")
	defer span.End()

	if _, err := r.db.Exec(ctx, markFailedQuery, id, lastError); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("notification.MarkFailed exec: %w", err)
	}
	return nil
}
