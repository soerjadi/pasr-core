package user

import (
	"context"
	"fmt"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (r *userRepository) UpsertByPhone(ctx context.Context, phone string, role domain.Role) (domain.User, error) {
	ctx, span := tracer.Start(ctx, "repository.user.UpsertByPhone")
	defer span.End()

	var u domain.User
	err := r.db.QueryRow(ctx, upsertByPhoneQuery, phone, string(role)).Scan(&u.ID, &u.Phone, &u.Name, &u.Role)
	if err != nil {
		tracer.RecordError(span, err)
		return domain.User{}, fmt.Errorf("user.UpsertByPhone query: %w", err)
	}
	return u, nil
}
