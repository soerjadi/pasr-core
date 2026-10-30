// Package admin implements the operator API. It holds no state machine of its own: order actions
// go through the OrderingService and notifications through the NotifyService.
package admin

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"
	"github.com/oemah/nusantara/core/internal/core/ports/services"
)

// Deps are the collaborators of the admin service.
type Deps struct {
	Stores     repository.StoreRepository
	Products   repository.ProductRepository
	Categories repository.CategoryRepository
	Ads        repository.AdRepository
	Notify     services.NotifyService
	Tx         repository.TxManager
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

// cleanText trims s and rejects NUL characters (PostgreSQL cannot store them) and more than maxRunes
// characters; required also rejects an empty result.
func cleanText(field, s string, maxRunes int, required bool) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case strings.ContainsRune(s, 0):
		return "", invalid(field + " must not contain NUL characters")
	case required && s == "":
		return "", invalid(field + " is required")
	case utf8.RuneCountInString(s) > maxRunes:
		return "", invalid(field + " must be at most " + strconv.Itoa(maxRunes) + " characters")
	}
	return s, nil
}
