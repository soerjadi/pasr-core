package category

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

const pgForeignKeyViolation = "23503"

func (r *categoryRepository) Create(ctx context.Context, name, iconKey string, sortOrder *int32) (domain.Category, error) {
	ctx, span := tracer.Start(ctx, "repository.category.Create")
	defer span.End()

	var c domain.Category
	if err := r.db.QueryRow(ctx, createQuery, name, iconKey, sortOrder).Scan(&c.ID, &c.Name, &c.IconKey); err != nil {
		tracer.RecordError(span, err)
		return domain.Category{}, fmt.Errorf("category.Create insert: %w", err)
	}
	return c, nil
}

func (r *categoryRepository) Update(ctx context.Context, id int64, name, iconKey *string, sortOrder *int32) (domain.Category, error) {
	ctx, span := tracer.Start(ctx, "repository.category.Update")
	defer span.End()

	var c domain.Category
	err := r.db.QueryRow(ctx, updateQuery, id, name, iconKey, sortOrder).Scan(&c.ID, &c.Name, &c.IconKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Category{}, notFound()
	}
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Category{}, fmt.Errorf("category.Update exec: %w", err)
	}
	return c, nil
}

func (r *categoryRepository) Delete(ctx context.Context, id int64) error {
	ctx, span := tracer.Start(ctx, "repository.category.Delete")
	defer span.End()

	tag, err := r.db.Exec(ctx, deleteQuery, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
			return domain.NewError(domain.ErrConflict, "CATEGORY_IN_USE", "the category is still used by stores or products")
		}
		tracer.RecordError(span, err)
		return fmt.Errorf("category.Delete exec: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return notFound()
	}
	return nil
}

func notFound() error {
	return domain.NewError(domain.ErrNotFound, "NOT_FOUND", "category not found")
}
