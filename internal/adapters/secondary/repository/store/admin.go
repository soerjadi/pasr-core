package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func scanAdmin(row pgx.Row) (domain.AdminStore, error) {
	var s domain.AdminStore
	var status string
	if err := row.Scan(&s.ID, &s.OwnerID, &s.Name, &s.OwnerName, &s.CategoryID, &s.Kelurahan, &s.Address,
		&s.WhatsApp, &status, &s.IsOpen, &s.CreatedAt); err != nil {
		return domain.AdminStore{}, err
	}
	s.Status = domain.StoreStatus(status)
	s.CreatedAt = s.CreatedAt.UTC()
	return s, nil
}

func (r *storeRepository) ListAdmin(ctx context.Context, q domain.AdminStoreQuery) ([]domain.AdminStore, error) {
	ctx, span := tracer.Start(ctx, "repository.store.ListAdmin")
	defer span.End()

	rows, err := r.db.Query(ctx, listAdminQuery, string(q.Status), q.BeforeID, q.Limit)
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("store.ListAdmin query: %w", err)
	}
	defer rows.Close()
	items := []domain.AdminStore{}
	for rows.Next() {
		s, err := scanAdmin(rows)
		if err != nil {
			tracer.RecordError(span, err)
			return nil, fmt.Errorf("store.ListAdmin scan: %w", err)
		}
		items = append(items, s)
	}
	if err := rows.Err(); err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("store.ListAdmin rows: %w", err)
	}
	return items, nil
}

func (r *storeRepository) GetAdmin(ctx context.Context, id int64, lock bool) (*domain.AdminStore, error) {
	ctx, span := tracer.Start(ctx, "repository.store.GetAdmin")
	defer span.End()

	q := getAdminQuery
	if lock {
		q = getAdminForUpdateQuery
	}
	s, err := scanAdmin(r.db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("store.GetAdmin query: %w", err)
	}
	return &s, nil
}

func (r *storeRepository) SetStatus(ctx context.Context, id int64, status domain.StoreStatus) error {
	ctx, span := tracer.Start(ctx, "repository.store.SetStatus")
	defer span.End()

	if _, err := r.db.Exec(ctx, setStatusQuery, id, string(status)); err != nil {
		tracer.RecordError(span, err)
		return fmt.Errorf("store.SetStatus exec: %w", err)
	}
	return nil
}
