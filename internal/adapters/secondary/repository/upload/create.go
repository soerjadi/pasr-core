package upload

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *uploadRepository) Create(ctx context.Context, ownerID int64, keys domain.ImageKeys) (int64, error) {
	ctx, span := tracer.Start(ctx, "repository.upload.Create")
	defer span.End()

	var id int64
	if err := r.db.QueryRow(ctx, createQuery, ownerID, keys).Scan(&id); err != nil {
		tracer.RecordError(span, err)
		return 0, fmt.Errorf("upload.Create insert: %w", err)
	}
	return id, nil
}
