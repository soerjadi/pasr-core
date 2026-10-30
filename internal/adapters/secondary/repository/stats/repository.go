package stats

import "github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"

type statsRepository struct {
	db repository.DB
}

func NewStatsRepository(db repository.DB) repository.StatsRepository {
	return &statsRepository{db: db}
}
