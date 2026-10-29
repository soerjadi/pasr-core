package notification

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *notificationRepository) Insert(ctx context.Context, n domain.Notification) (int64, error) {
	ctx, span := tracer.Start(ctx, "repository.notification.Insert")
	defer span.End()

	data := n.Data
	if data == nil {
		data = map[string]string{}
	}
	raw, err := json.Marshal(data)
	if err != nil {
		tracer.RecordError(span, err)
		return 0, fmt.Errorf("notification.Insert marshal data: %w", err)
	}
	var id int64
	if err := r.db.QueryRow(ctx, insertQuery, n.UserID, n.Kind, n.Title, n.Body, raw).Scan(&id); err != nil {
		tracer.RecordError(span, err)
		return 0, fmt.Errorf("notification.Insert exec: %w", err)
	}
	return id, nil
}
