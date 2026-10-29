package notification

import "github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"

type notificationRepository struct {
	db repository.DB
}

func NewNotificationRepository(db repository.DB) repository.NotificationRepository {
	return &notificationRepository{db: db}
}
