package services

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type IdentityService interface {
	RequestOTP(ctx context.Context, phone, ip string) error
	VerifyOTP(ctx context.Context, phone, code string) (domain.Session, error)
	Refresh(ctx context.Context, refreshToken string) (domain.Session, error)
	// Logout revokes the refresh token if it is known; it never reveals whether it was.
	Logout(ctx context.Context, refreshToken string) error
	ParseAccessToken(token string) (domain.Claims, error)
}
