package category

import "github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"

type categoryRepository struct {
	db repository.DB
}

func NewCategoryRepository(db repository.DB) repository.CategoryRepository {
	return &categoryRepository{db: db}
}
