package repository

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type UserRepository interface {
	// UpsertByPhone creates the user or updates its role to the given one, and returns it.
	UpsertByPhone(ctx context.Context, phone string, role domain.Role) (domain.User, error)
	// GetByID returns domain.ErrNotFound for an unknown id.
	GetByID(ctx context.Context, id int64) (domain.User, error)
}
