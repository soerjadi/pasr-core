package domain

import "time"

// Session matches the Session schema in docs/api/openapi.yaml.
type Session struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	User         User   `json:"user"`
}

// OTPCode is a stored one-time code; only its keyed hash is kept.
type OTPCode struct {
	ID        int64
	Phone     string
	CodeHash  string
	ExpiresAt time.Time
	Attempts  int
}

// RefreshToken is a stored refresh token; only its keyed hash is kept.
// All tokens issued by rotation from one login share FamilyID.
type RefreshToken struct {
	ID        int64
	UserID    int64
	FamilyID  string
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
}
