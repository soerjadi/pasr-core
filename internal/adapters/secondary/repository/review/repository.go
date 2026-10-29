package review

import "github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"

type reviewRepository struct {
	db repository.DB
}

func NewReviewRepository(db repository.DB) repository.ReviewRepository {
	return &reviewRepository{db: db}
}
