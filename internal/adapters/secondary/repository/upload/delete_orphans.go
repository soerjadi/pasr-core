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

	out, err := r.deleteReturningKeys(ctx, "upload.DeleteOrphans", deleteOrphansQuery, olderThan)
	if err != nil {
		tracer.RecordError(span, err)
	}
	return out, err
}

func (r *uploadRepository) DeleteDetached(ctx context.Context, olderThan time.Time) ([]domain.ImageKeys, error) {
	ctx, span := tracer.Start(ctx, "repository.upload.DeleteDetached")
	defer span.End()

	out, err := r.deleteReturningKeys(ctx, "upload.DeleteDetached", deleteDetachedQuery, olderThan)
	if err != nil {
		tracer.RecordError(span, err)
	}
	return out, err
}

// deleteReturningKeys runs a DELETE ... RETURNING keys and collects the keys.
func (r *uploadRepository) deleteReturningKeys(ctx context.Context, op, query string, olderThan time.Time) ([]domain.ImageKeys, error) {
	rows, err := r.db.Query(ctx, query, olderThan)
	if err != nil {
		return nil, fmt.Errorf("%s query: %w", op, err)
	}
	defer rows.Close()
	out := []domain.ImageKeys{}
	for rows.Next() {
		var keys domain.ImageKeys
		if err := rows.Scan(&keys); err != nil {
			return nil, fmt.Errorf("%s scan: %w", op, err)
		}
		out = append(out, keys)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s rows: %w", op, err)
	}
	return out, nil
}
