package device

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *deviceRepository) ListTokens(ctx context.Context, userID int64) ([]string, error) {
	ctx, span := tracer.Start(ctx, "repository.device.ListTokens")
	defer span.End()

	rows, err := r.db.Query(ctx, listTokensQuery, userID)
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("device.ListTokens query: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var tok string
		if err := rows.Scan(&tok); err != nil {
			tracer.RecordError(span, err)
			return nil, fmt.Errorf("device.ListTokens scan: %w", err)
		}
		out = append(out, tok)
	}
	if err := rows.Err(); err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("device.ListTokens rows: %w", err)
	}
	return out, nil
}
