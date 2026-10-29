package product

import "github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"

type productRepository struct {
	db repository.DB
}

func NewProductRepository(db repository.DB) repository.ProductRepository {
	return &productRepository{db: db}
}
