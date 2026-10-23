package auth

import (
	"time"

	"github.com/ABUDIYAAAA/benchmarq/pkg/model"
	"github.com/google/uuid"
)

type User struct {
	model.Model
	Email           string  `json:"email" db:"email"`
	PasswordHash    *string `json:"-" db:"password_hash"`
	FirstName       string  `json:"first_name" db:"first_name"`
	LastName        *string `json:"last_name,omitempty" db:"last_name"`
	AvatarURL       *string `json:"avatar_url,omitempty" db:"avatar_url"`
	IsActive        bool    `json:"is_active" db:"is_active"`
	IsEmailVerified bool    `json:"is_email_verified" db:"is_email_verified"`
}

type UserSession struct {
	model.Model
	UserID           uuid.UUID `json:"user_id" db:"user_id"`
	DeviceID         string    `json:"device_id" db:"device_id"`
	RefreshTokenHash string    `json:"-" db:"refresh_token_hash"`
	UserAgent        *string   `json:"user_agent,omitempty" db:"user_agent"`
	IPAddress        *string   `json:"ip_address,omitempty" db:"ip_address"`
	IsRevoked        bool      `json:"is_revoked" db:"is_revoked"`
	ExpiresAt        time.Time `json:"expires_at" db:"expires_at"`
	// PreviousRefreshTokenHash is the token this session last rotated away
	// from, at RotatedAt.
	PreviousRefreshTokenHash *string    `json:"-" db:"previous_refresh_token_hash"`
	RotatedAt                *time.Time `json:"-" db:"rotated_at"`
}

type EmailVerificationToken struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	TokenHash string    `json:"-" db:"token_hash"`
	IsUsed    bool      `json:"is_used" db:"is_used"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type PasswordResetToken struct {
	model.Model
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	TokenHash string    `json:"-" db:"token_hash"`
	IsUsed    bool      `json:"is_used" db:"is_used"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
}
