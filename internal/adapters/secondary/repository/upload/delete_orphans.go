package upload

import (
	"context"
	"fmt"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *uploadRepository) DeleteOrphans(ctx context.Context, olderThan time.Time) ([]domain.ImageKeys, error) {
	ctx, span := tracer.Start(ctx, "repository.upload.DeleteOrphans")
	defer span.End()

	rows, err := r.db.Query(ctx, deleteOrphansQuery, olderThan)
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("upload.DeleteOrphans query: %w", err)
	}
	defer rows.Close()
	out := []domain.ImageKeys{}
	for rows.Next() {
		var keys domain.ImageKeys
		if err := rows.Scan(&keys); err != nil {
			tracer.RecordError(span, err)
			return nil, fmt.Errorf("upload.DeleteOrphans scan: %w", err)
		}
		out = append(out, keys)
	}
	if err := rows.Err(); err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("upload.DeleteOrphans rows: %w", err)
	}
	return out, nil
}
