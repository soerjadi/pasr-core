package user

import "github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"

type userRepository struct {
	db repository.DB
}

func NewUserRepository(db repository.DB) repository.UserRepository {
	return &userRepository{db: db}
}
