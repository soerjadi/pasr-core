package identity

import (
	"context"
	"errors"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/infrastructure/tracer"
)

func (s *service) Refresh(ctx context.Context, refreshToken string) (domain.Session, error) {
	ctx, span := tracer.Start(ctx, "service.identity.Refresh")
	defer span.End()

	var (
		session domain.Session
		denied  error // returned after the transaction commits, so a family revocation is kept
	)
	now := s.clock.Now()
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		old, err := s.sessions.GetByHash(ctx, hashHMAC(s.cfg.RefreshSecret, refreshToken))
		if errors.Is(err, domain.ErrNotFound) {
			denied = domain.ErrUnauthorized
			return nil
		}
		if err != nil {
			return err
		}
		if old.RevokedAt != nil {
			denied = domain.ErrUnauthorized
			return s.sessions.RevokeFamily(ctx, old.FamilyID, now) // reuse: kill the whole family
		}
		if !now.Before(old.ExpiresAt) {
			denied = domain.ErrUnauthorized
			return nil
		}
		revoked, err := s.sessions.Revoke(ctx, old.ID, now)
		if err != nil {
			return err
		}
		if !revoked { // a concurrent refresh used it first: treat as reuse
			denied = domain.ErrUnauthorized
			return s.sessions.RevokeFamily(ctx, old.FamilyID, now)
		}
		user, err := s.users.GetByID(ctx, old.UserID)
		if errors.Is(err, domain.ErrNotFound) {
			denied = domain.ErrUnauthorized
			return nil
		}
		if err != nil {
			return err
		}
		session, err = s.issueSession(ctx, user, old.FamilyID, now)
		return err
	})
	if err != nil {
		tracer.RecordError(span, err)
		return domain.Session{}, err
	}
	if denied != nil {
		return domain.Session{}, denied
	}
	return session, nil
}
