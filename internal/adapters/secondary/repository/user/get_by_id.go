package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *userRepository) GetByID(ctx context.Context, id int64) (domain.User, error) {
	ctx, span := tracer.Start(ctx, "repository.user.GetByID")
	defer span.End()

	var u domain.User
	err := r.db.QueryRow(ctx, getByIDQuery, id).Scan(&u.ID, &u.Phone, &u.Name, &u.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	if err != nil {
		tracer.RecordError(span, err)
		return domain.User{}, fmt.Errorf("user.GetByID query: %w", err)
	}
	return u, nil
}
