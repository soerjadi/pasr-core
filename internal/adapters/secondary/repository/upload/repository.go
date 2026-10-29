package upload

import "github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"

type uploadRepository struct {
	db repository.DB
}

func NewUploadRepository(db repository.DB) repository.UploadRepository {
	return &uploadRepository{db: db}
}
