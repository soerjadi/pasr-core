package storefront

import "github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"

type storefrontRepository struct {
	db repository.DB
}

func NewStorefrontRepository(db repository.DB) repository.StorefrontRepository {
	return &storefrontRepository{db: db}
}
