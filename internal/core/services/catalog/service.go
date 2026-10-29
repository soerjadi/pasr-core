package catalog

import (
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"
	"github.com/oemah/nusantara/core/internal/core/ports/services"
)

type service struct {
	categories repository.CategoryRepository
}

func NewService(categories repository.CategoryRepository) services.CatalogService {
	return &service{categories: categories}
}
