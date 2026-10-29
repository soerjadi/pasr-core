package notification

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *notificationRepository) ClaimUnsent(ctx context.Context, limit, maxAttempts int) ([]domain.Notification, error) {
	ctx, span := tracer.Start(ctx, "repository.notification.ClaimUnsent")
	defer span.End()

	rows, err := r.db.Query(ctx, claimUnsentQuery, limit, maxAttempts)
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("notification.ClaimUnsent query: %w", err)
	}
	defer rows.Close()
	var out []domain.Notification
	for rows.Next() {
		var n domain.Notification
		var raw []byte
		if err := rows.Scan(&n.ID, &n.UserID, &n.Kind, &n.Title, &n.Body, &raw); err != nil {
			tracer.RecordError(span, err)
			return nil, fmt.Errorf("notification.ClaimUnsent scan: %w", err)
		}
		if err := json.Unmarshal(raw, &n.Data); err != nil {
			tracer.RecordError(span, err)
			return nil, fmt.Errorf("notification.ClaimUnsent decode data: %w", err)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("notification.ClaimUnsent rows: %w", err)
	}
	return out, nil
}
