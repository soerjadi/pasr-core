package otp

import "github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"

type otpRepository struct {
	db repository.DB
}

func NewOTPRepository(db repository.DB) repository.OTPRepository {
	return &otpRepository{db: db}
}
