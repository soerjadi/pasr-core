package social

import "github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"

type socialRepository struct {
	db repository.DB
}

func NewSocialRepository(db repository.DB) repository.SocialRepository {
	return &socialRepository{db: db}
}
