package identity

import (
	"context"
	"errors"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) Logout(ctx context.Context, refreshToken string) error {
	ctx, span := tracer.Start(ctx, "service.identity.Logout")
	defer span.End()

	t, err := s.sessions.GetByHash(ctx, hashHMAC(s.cfg.RefreshSecret, refreshToken))
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		tracer.RecordError(span, err)
		return err
	}
	if _, err := s.sessions.Revoke(ctx, t.ID, s.clock.Now()); err != nil {
		tracer.RecordError(span, err)
		return err
	}
	return nil
}
