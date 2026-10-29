package identity

import (
	"context"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

// issueSession signs an access token and stores a fresh refresh token in the given family.
func (s *service) issueSession(ctx context.Context, user domain.User, familyID string, now time.Time) (domain.Session, error) {
	access, _, err := s.tokens.Issue(domain.Claims{UserID: user.ID, Role: user.Role})
	if err != nil {
		return domain.Session{}, err
	}
	refresh, err := newRefreshToken()
	if err != nil {
		return domain.Session{}, err
	}
	err = s.sessions.Create(ctx, domain.RefreshToken{
		UserID:    user.ID,
		FamilyID:  familyID,
		TokenHash: hashHMAC(s.cfg.RefreshSecret, refresh),
		ExpiresAt: now.Add(s.cfg.RefreshTTL),
	}, now)
	if err != nil {
		return domain.Session{}, err
	}
	return domain.Session{AccessToken: access, RefreshToken: refresh, User: user}, nil
}
