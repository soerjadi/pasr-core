package ad

import "github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"

type adRepository struct {
	db repository.DB
}

func NewAdRepository(db repository.DB) repository.AdRepository {
	return &adRepository{db: db}
}
