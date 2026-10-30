// Package admin implements the operator API. It holds no state machine of its own: order actions
// go through the OrderingService and notifications through the NotifyService.
package admin

import (
	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"
	"github.com/oemah/nusantara/core/internal/core/ports/services"
)

// Deps are the collaborators of the admin service.
type Deps struct {
	Stores repository.StoreRepository
	Notify services.NotifyService
	Tx     repository.TxManager
}

type service struct {
	Deps
}

func NewService(d Deps) services.AdminService {
	return &service{Deps: d}
}

func invalid(msg string) error {
	return domain.NewError(domain.ErrValidationFailed, "VALIDATION_FAILED", msg)
}
