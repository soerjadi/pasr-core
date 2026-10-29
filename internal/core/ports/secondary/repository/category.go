package repository

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type CategoryRepository interface {
	GetAll(ctx context.Context) ([]domain.Category, error)
}
