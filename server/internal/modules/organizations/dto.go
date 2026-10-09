package organizations

import (
	"time"

	"github.com/ABUDIYAAAA/benchmarq/internal/authz"
	"github.com/google/uuid"
)

// OrganizationInput is the writable view of an organization. It is the body of
// create requests and the merge target of PATCH requests.
type OrganizationInput struct {
	Name     string         `json:"name" validate:"required,min=2,max=255"`
	Slug     string         `json:"slug" validate:"omitempty,min=3,max=100,slug"`
	LogoURL  *string        `json:"logo_url" validate:"omitempty,http_url,max=2048"`
	Website  *string        `json:"website" validate:"omitempty,http_url,max=2048"`
	Branding map[string]any `json:"branding"`
}

func inputFrom(o *Organization) OrganizationInput {
	return OrganizationInput{
		Name:     o.Name,
		Slug:     o.Slug,
		LogoURL:  o.LogoURL,
		Website:  o.Website,
		Branding: o.Branding,
	}
}

type UpdateMemberRoleRequest struct {
	Role authz.Role `json:"role" validate:"required,oneof=owner admin instructor proctor candidate"`
}

type OrganizationResponse struct {
	ID        uuid.UUID      `json:"id"`
	Name      string         `json:"name"`
	Slug      string         `json:"slug"`
	LogoURL   *string        `json:"logo_url,omitempty"`
	Website   *string        `json:"website,omitempty"`
	Branding  map[string]any `json:"branding"`
	MyRole    authz.Role     `json:"my_role"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func toResponse(o *Organization, role authz.Role) *OrganizationResponse {
	branding := o.Branding
	if branding == nil {
		branding = map[string]any{}
	}
	return &OrganizationResponse{
		ID:        o.ID,
		Name:      o.Name,
		Slug:      o.Slug,
		LogoURL:   o.LogoURL,
		Website:   o.Website,
		Branding:  branding,
		MyRole:    role,
		CreatedAt: o.CreatedAt,
		UpdatedAt: o.UpdatedAt,
	}
}
