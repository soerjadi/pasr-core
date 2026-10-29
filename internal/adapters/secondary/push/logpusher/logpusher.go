// Package logpusher is the development push sender: it logs instead of contacting a provider.
package logpusher

import (
	"context"

	log "github.com/oemahdev/logger"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/push"
)

type pusher struct{}

func New() push.Pusher { return pusher{} }

func (pusher) Send(ctx context.Context, token string, n domain.Notification) (bool, error) {
	log.InfoCtx(ctx, "push: notification (log pusher, nothing sent)",
		log.Field("notificationId", n.ID), log.Field("kind", n.Kind), log.Field("token", Mask(token)))
	return false, nil
}

// Mask keeps only the last four characters of a device token.
func Mask(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return "****" + token[len(token)-4:]
}
