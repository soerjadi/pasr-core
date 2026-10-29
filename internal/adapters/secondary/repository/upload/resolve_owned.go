package upload

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *uploadRepository) ResolveOwned(ctx context.Context, ownerID int64, ids []int64) (map[int64]domain.ImageKeys, error) {
	ctx, span := tracer.Start(ctx, "repository.upload.ResolveOwned")
	defer span.End()

	out := make(map[int64]domain.ImageKeys, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, resolveOwnedQuery, ownerID, ids)
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("upload.ResolveOwned query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id   int64
			keys domain.ImageKeys
		)
		if err := rows.Scan(&id, &keys); err != nil {
			tracer.RecordError(span, err)
			return nil, fmt.Errorf("upload.ResolveOwned scan: %w", err)
		}
		out[id] = keys
	}
	if err := rows.Err(); err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("upload.ResolveOwned rows: %w", err)
	}
	for _, id := range ids {
		if _, ok := out[id]; !ok {
			// Same answer for "missing", "someone else's" and "already used": no existence oracle.
			return nil, domain.NewError(domain.ErrValidationFailed, "VALIDATION_FAILED", "unknown or already used image")
		}
	}
	return out, nil
}
