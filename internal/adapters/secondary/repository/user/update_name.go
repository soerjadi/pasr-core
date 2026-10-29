package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *userRepository) UpdateName(ctx context.Context, id int64, name string) (domain.User, error) {
	ctx, span := tracer.Start(ctx, "repository.user.UpdateName")
	defer span.End()

	var u domain.User
	err := r.db.QueryRow(ctx, updateNameQuery, id, name).Scan(&u.ID, &u.Phone, &u.Name, &u.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	if err != nil {
		tracer.RecordError(span, err)
		return domain.User{}, fmt.Errorf("user.UpdateName query: %w", err)
	}
	return u, nil
}
