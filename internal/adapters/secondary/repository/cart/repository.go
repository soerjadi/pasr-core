package cart

import "github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"

type cartRepository struct {
	db repository.DB
}

func NewCartRepository(db repository.DB) repository.CartRepository {
	return &cartRepository{db: db}
}
