package store

import "github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"

type storeRepository struct {
	db repository.DB
}

func NewStoreRepository(db repository.DB) repository.StoreRepository {
	return &storeRepository{db: db}
}
