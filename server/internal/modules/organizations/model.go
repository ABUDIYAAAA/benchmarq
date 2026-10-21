package organizations

import (
	"time"

	"github.com/ABUDIYAAAA/benchmarq/internal/authz"
	"github.com/ABUDIYAAAA/benchmarq/pkg/model"
	"github.com/google/uuid"
)

type Organization struct {
	model.Model
	Name            string         `json:"name" db:"name"`
	Slug            string         `json:"slug" db:"slug"`
	LogoURL         *string        `json:"logo_url,omitempty" db:"logo_url"`
	Website         *string        `json:"website,omitempty" db:"website"`
	Branding        map[string]any `json:"branding" db:"branding"`
	CreatedByUserID *uuid.UUID     `json:"created_by_user_id,omitempty" db:"created_by_user_id"`
}

// Membership pairs an organization with the viewing user's role in it.
type Membership struct {
	Organization Organization
	Role         authz.Role
	JoinedAt     time.Time
}

type Member struct {
	UserID    uuid.UUID  `json:"user_id"`
	Email     string     `json:"email"`
	FirstName string     `json:"first_name"`
	LastName  *string    `json:"last_name,omitempty"`
	AvatarURL *string    `json:"avatar_url,omitempty"`
	Role      authz.Role `json:"role"`
	JoinedAt  time.Time  `json:"joined_at"`
}

type InvitationStatus string

const (
	InvitationPending  InvitationStatus = "pending"
	InvitationAccepted InvitationStatus = "accepted"
	InvitationExpired  InvitationStatus = "expired"
	InvitationRevoked  InvitationStatus = "revoked"
)

// Invitation asks someone, by email, to join an organization with a role.
// A pending invitation past its expiry is reported as expired.
type Invitation struct {
	ID              uuid.UUID        `json:"id"`
	OrgID           uuid.UUID        `json:"org_id"`
	Email           string           `json:"email"`
	Role            authz.Role       `json:"role"`
	Status          InvitationStatus `json:"status"`
	InvitedByUserID *uuid.UUID       `json:"invited_by_user_id,omitempty"`
	ExpiresAt       time.Time        `json:"expires_at"`
	AcceptedAt      *time.Time       `json:"accepted_at,omitempty"`
	RevokedAt       *time.Time       `json:"revoked_at,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`

	TokenHash string `json:"-"`
}
