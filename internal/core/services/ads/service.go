package ads

import (
	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"
	"github.com/oemah/nusantara/core/internal/core/ports/services"
)

type service struct {
	ads   repository.AdRepository
	clock domain.Clock
}

func NewService(ads repository.AdRepository, clock domain.Clock) services.AdsService {
	return &service{ads: ads, clock: clock}
}
