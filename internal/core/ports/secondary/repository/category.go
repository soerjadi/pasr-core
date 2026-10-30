package repository

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type CategoryRepository interface {
	GetAll(ctx context.Context) ([]domain.Category, error)
	// Create inserts a category; a nil sortOrder places it after the last one.
	Create(ctx context.Context, name, iconKey string, sortOrder *int32) (domain.Category, error)
	// Update changes the non-nil fields and returns the row, or domain.ErrNotFound.
	Update(ctx context.Context, id int64, name, iconKey *string, sortOrder *int32) (domain.Category, error)
	// Delete removes an unused category. One a store or a product (deleted ones included) still
	// refers to is a coded domain.ErrConflict "CATEGORY_IN_USE"; an unknown id domain.ErrNotFound.
	Delete(ctx context.Context, id int64) error
}
