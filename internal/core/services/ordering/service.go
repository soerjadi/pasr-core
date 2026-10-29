package ordering

import (
	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/storage"
	"github.com/oemah/nusantara/core/internal/core/ports/services"
)

// MaxCartLines is the most distinct lines a cart request may carry.
const MaxCartLines = 50

type service struct {
	carts   repository.CartRepository
	storage storage.ObjectStorage
}

func NewService(carts repository.CartRepository, objects storage.ObjectStorage) services.OrderingService {
	return &service{carts: carts, storage: objects}
}

func invalid(msg string) error {
	return domain.NewError(domain.ErrValidationFailed, "VALIDATION_FAILED", msg)
}

func (s *service) imageSet(k domain.ImageKeys) domain.ImageSet {
	return domain.ImageSet{Small: s.storage.URL(k.Small), Medium: s.storage.URL(k.Medium), Large: s.storage.URL(k.Large)}
}
