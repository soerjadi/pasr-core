package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
)

func (r *storeRepository) Create(ctx context.Context, s domain.Store) (domain.StoreSummary, error) {
	ctx, span := tracer.Start(ctx, "repository.store.Create")
	defer span.End()

	var out domain.StoreSummary
	err := r.db.QueryRow(ctx, createQuery,
		s.OwnerID, s.Name, s.CategoryID, s.OwnerName, s.Kelurahan, s.Address, s.WhatsApp, s.PhotoUploadID,
	).Scan(&out.ID, &out.Name, &out.Status)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch {
			case pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == "stores_owner_id_key":
				return domain.StoreSummary{}, domain.NewError(domain.ErrConflict, "STORE_EXISTS", "you already have a store")
			case pgErr.Code == pgForeignKeyViolation && pgErr.ConstraintName == "stores_category_id_fkey":
				return domain.StoreSummary{}, domain.NewError(domain.ErrValidationFailed, "VALIDATION_FAILED", "unknown category")
			}
		}
		tracer.RecordError(span, err)
		return domain.StoreSummary{}, fmt.Errorf("store.Create insert: %w", err)
	}
	return out, nil
}
