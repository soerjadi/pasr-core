package account

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"
	"github.com/oemah/nusantara/core/internal/core/ports/services"
)

type service struct {
	users   repository.UserRepository
	stores  repository.StoreRepository
	devices repository.DeviceRepository
	uploads repository.UploadRepository
	tx      repository.TxManager
}

func NewService(
	users repository.UserRepository,
	stores repository.StoreRepository,
	devices repository.DeviceRepository,
	uploads repository.UploadRepository,
	tx repository.TxManager,
) services.AccountService {
	return &service{users: users, stores: stores, devices: devices, uploads: uploads, tx: tx}
}

func invalid(msg string) error {
	return domain.NewError(domain.ErrValidationFailed, "VALIDATION_FAILED", msg)
}

// requireRunes trims v and checks its length in runes against 1..max.
func requireRunes(field, v string, max int) (string, error) {
	v = strings.TrimSpace(v)
	if n := utf8.RuneCountInString(v); n < 1 || n > max {
		return "", invalid(field + " must be between 1 and " + strconv.Itoa(max) + " characters")
	}
	return v, nil
}
