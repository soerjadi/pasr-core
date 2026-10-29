package device

import "github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"

type deviceRepository struct {
	db repository.DB
}

func NewDeviceRepository(db repository.DB) repository.DeviceRepository {
	return &deviceRepository{db: db}
}
