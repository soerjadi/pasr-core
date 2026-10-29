package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *storeRepository) Update(ctx context.Context, storeID int64, u domain.StoreUpdate) error {
	ctx, span := tracer.Start(ctx, "repository.store.Update")
	defer span.End()

	var hours []byte
	if u.Hours != nil {
		b, err := json.Marshal(u.Hours)
		if err != nil {
			tracer.RecordError(span, err)
			return fmt.Errorf("store.Update marshal hours: %w", err)
		}
		hours = b
	}
	_, err := r.db.Exec(ctx, updateQuery, storeID,
		u.Name, u.CategoryID, u.OwnerName, u.Kelurahan, u.Address, u.Description, u.PhotoUploadID, u.WhatsApp, hours)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation && pgErr.ConstraintName == "stores_category_id_fkey" {
			return domain.NewError(domain.ErrValidationFailed, "VALIDATION_FAILED", "unknown category")
		}
		tracer.RecordError(span, err)
		return fmt.Errorf("store.Update exec: %w", err)
	}
	return nil
}
