package session

import "github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"

type sessionRepository struct {
	db repository.DB
}

func NewSessionRepository(db repository.DB) repository.SessionRepository {
	return &sessionRepository{db: db}
}
