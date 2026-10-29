package upload

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *uploadRepository) MarkAttached(ctx context.Context, ids []int64) error {
	ctx, span := tracer.Start(ctx, "repository.upload.MarkAttached")
	defer span.End()

	if len(ids) == 0 {
		return nil
	}
	if _, err := r.db.Exec(ctx, markAttachedQuery, ids); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("upload.MarkAttached exec: %w", err)
	}
	return nil
}
