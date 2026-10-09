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
