package idempotency

import "github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"

type idempotencyRepository struct {
	db repository.DB
}

func NewIdempotencyRepository(db repository.DB) repository.IdempotencyRepository {
	return &idempotencyRepository{db: db}
}
