package services

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type CatalogService interface {
	ListCategories(ctx context.Context) ([]domain.Category, error)
}
