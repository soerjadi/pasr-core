package identity

import (
	"errors"
	"fmt"
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/otp"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"
	"github.com/oemah/nusantara/core/internal/core/ports/services"
)

// Config holds the identity settings. Every secret must be non-empty.
type Config struct {
	AccessSecret   string
	AccessTTL      time.Duration
	RefreshSecret  string
	RefreshTTL     time.Duration
	OTPSecret      string
	OTPTTL         time.Duration
	OTPMaxAttempts int
	OTPPhoneLimit  int
	OTPIPLimit     int
	OTPWindow      time.Duration
	OperatorPhones []string
}

type service struct {
	users     repository.UserRepository
	otps      repository.OTPRepository
	sessions  repository.SessionRepository
	tx        repository.TxManager
	sender    otp.Sender
	clock     domain.Clock
	tokens    *Tokens
	cfg       Config
	operators map[string]bool
}

// NewService refuses to build with an empty secret or a non-positive limit, so tokens and hashes
// can never be keyed with nothing.
func NewService(
	users repository.UserRepository,
	otps repository.OTPRepository,
	sessions repository.SessionRepository,
	tx repository.TxManager,
	sender otp.Sender,
	clock domain.Clock,
	cfg Config,
) (services.IdentityService, error) {
	switch {
	case cfg.AccessSecret == "":
		return nil, errors.New("identity: jwt.access_secret must not be empty")
	case cfg.RefreshSecret == "":
		return nil, errors.New("identity: jwt.refresh_secret must not be empty")
	case cfg.OTPSecret == "":
		return nil, errors.New("identity: otp.secret must not be empty")
	case cfg.AccessTTL <= 0 || cfg.RefreshTTL <= 0 || cfg.OTPTTL <= 0 || cfg.OTPWindow <= 0:
		return nil, errors.New("identity: token, otp ttl and window must be positive")
	case cfg.OTPMaxAttempts <= 0 || cfg.OTPPhoneLimit <= 0 || cfg.OTPIPLimit <= 0:
		return nil, errors.New("identity: otp attempt and rate limits must be positive")
	}
	operators := make(map[string]bool, len(cfg.OperatorPhones))
	for _, p := range cfg.OperatorPhones {
		n, err := domain.NormalizePhone(p)
		if err != nil {
			return nil, fmt.Errorf("identity: app.operator_phones has an invalid entry: %w", err)
		}
		operators[n] = true
	}
	return &service{
		users: users, otps: otps, sessions: sessions, tx: tx, sender: sender, clock: clock,
		tokens: NewTokens(cfg.AccessSecret, cfg.AccessTTL, clock), cfg: cfg, operators: operators,
	}, nil
}

func (s *service) ParseAccessToken(token string) (domain.Claims, error) {
	return s.tokens.Parse(token)
}
