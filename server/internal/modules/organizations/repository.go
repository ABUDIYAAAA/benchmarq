package organizations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ABUDIYAAAA/benchmarq/internal/authz"
	"github.com/ABUDIYAAAA/benchmarq/internal/database"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/pagination"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrOrganizationNotFound = errors.New("organization not found")
	ErrSlugTaken            = errors.New("organization slug is already taken")
	ErrMemberNotFound       = errors.New("member not found")
)

type Repository interface {
	authz.MembershipResolver

	Create(ctx context.Context, org *Organization) error
	GetByID(ctx context.Context, id uuid.UUID) (*Organization, error)
	// LockByID fetches the organization with a row lock held until the
	// surrounding transaction ends. Used to serialize membership changes.
	LockByID(ctx context.Context, id uuid.UUID) (*Organization, error)
	Update(ctx context.Context, org *Organization) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	ListForUser(ctx context.Context, userID uuid.UUID, p pagination.Params) ([]Membership, error)

	AddMember(ctx context.Context, orgID, userID uuid.UUID, role authz.Role) error
	GetMember(ctx context.Context, orgID, userID uuid.UUID) (*Member, error)
	ListMembers(ctx context.Context, orgID uuid.UUID, p pagination.Params) ([]Member, error)
	CountOwners(ctx context.Context, orgID uuid.UUID) (int, error)
	UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role authz.Role) error
	RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error
}

type postgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &postgresRepository{db: db}
}

const orgColumns = `id, name, slug, logo_url, website, branding, created_by_user_id, created_at, updated_at, deleted_at`

func scanOrg(row interface{ Scan(...any) error }, o *Organization) error {
	return row.Scan(&o.ID, &o.Name, &o.Slug, &o.LogoURL, &o.Website, &o.Branding,
		&o.CreatedByUserID, &o.CreatedAt, &o.UpdatedAt, &o.DeletedAt)
}

func (r *postgresRepository) Create(ctx context.Context, org *Organization) error {
	if org.ID == uuid.Nil {
		org.ID = uuid.New()
	}
	if org.Branding == nil {
		org.Branding = map[string]any{}
	}
	// ON CONFLICT DO NOTHING (rather than catching the unique violation) keeps
	// the surrounding transaction usable, so the caller can retry another slug.
	query := `
		INSERT INTO organizations (id, name, slug, logo_url, website, branding, created_by_user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (slug) DO NOTHING
		RETURNING created_at, updated_at
	`
	err := database.Conn(ctx, r.db).QueryRow(ctx, query,
		org.ID, org.Name, org.Slug, org.LogoURL, org.Website, org.Branding, org.CreatedByUserID,
	).Scan(&org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		if database.IsNoRows(err) {
			return ErrSlugTaken
		}
		return fmt.Errorf("insert organization: %w", err)
	}
	return nil
}

func (r *postgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*Organization, error) {
	return r.get(ctx, id, false)
}

func (r *postgresRepository) LockByID(ctx context.Context, id uuid.UUID) (*Organization, error) {
	return r.get(ctx, id, true)
}

func (r *postgresRepository) get(ctx context.Context, id uuid.UUID, forUpdate bool) (*Organization, error) {
	query := `SELECT ` + orgColumns + ` FROM organizations WHERE id = $1 AND deleted_at IS NULL`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var org Organization
	if err := scanOrg(database.Conn(ctx, r.db).QueryRow(ctx, query, id), &org); err != nil {
		if database.IsNoRows(err) {
			return nil, ErrOrganizationNotFound
		}
		return nil, fmt.Errorf("select organization: %w", err)
	}
	return &org, nil
}

func (r *postgresRepository) Update(ctx context.Context, org *Organization) error {
	if org.Branding == nil {
		org.Branding = map[string]any{}
	}
	query := `
		UPDATE organizations
		SET name = $2, slug = $3, logo_url = $4, website = $5, branding = $6, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING updated_at
	`
	err := database.Conn(ctx, r.db).QueryRow(ctx, query,
		org.ID, org.Name, org.Slug, org.LogoURL, org.Website, org.Branding,
	).Scan(&org.UpdatedAt)
	if err != nil {
		if database.IsNoRows(err) {
			return ErrOrganizationNotFound
		}
		if database.IsUniqueViolation(err, "organizations_slug_key") {
			return ErrSlugTaken
		}
		return fmt.Errorf("update organization: %w", err)
	}
	return nil
}

func (r *postgresRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	tag, err := database.Conn(ctx, r.db).Exec(ctx,
		`UPDATE organizations SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft delete organization: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrOrganizationNotFound
	}
	return nil
}

func (r *postgresRepository) ListForUser(ctx context.Context, userID uuid.UUID, p pagination.Params) ([]Membership, error) {
	args := []any{userID, p.Limit + 1}
	cursorClause := ""
	if p.After != nil {
		cursorClause = `AND (m.joined_at, o.id) < ($3, $4)`
		args = append(args, p.After.Time, p.After.ID)
	}
	query := `
		SELECT o.id, o.name, o.slug, o.logo_url, o.website, o.branding, o.created_by_user_id,
		       o.created_at, o.updated_at, o.deleted_at, m.role, m.joined_at
		FROM organization_members m
		JOIN organizations o ON o.id = m.org_id AND o.deleted_at IS NULL
		WHERE m.user_id = $1 ` + cursorClause + `
		ORDER BY m.joined_at DESC, o.id DESC
		LIMIT $2
	`
	rows, err := database.Conn(ctx, r.db).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list organizations for user: %w", err)
	}
	defer rows.Close()

	var out []Membership
	for rows.Next() {
		var m Membership
		o := &m.Organization
		if err := rows.Scan(&o.ID, &o.Name, &o.Slug, &o.LogoURL, &o.Website, &o.Branding, &o.CreatedByUserID,
			&o.CreatedAt, &o.UpdatedAt, &o.DeletedAt, &m.Role, &m.JoinedAt); err != nil {
			return nil, fmt.Errorf("scan organization membership: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *postgresRepository) ResolveRole(ctx context.Context, orgID, userID uuid.UUID) (authz.Role, error) {
	query := `
		SELECT m.role
		FROM organization_members m
		JOIN organizations o ON o.id = m.org_id AND o.deleted_at IS NULL
		WHERE m.org_id = $1 AND m.user_id = $2
	`
	var role authz.Role
	if err := database.Conn(ctx, r.db).QueryRow(ctx, query, orgID, userID).Scan(&role); err != nil {
		if database.IsNoRows(err) {
			return "", authz.ErrNotMember
		}
		return "", fmt.Errorf("resolve membership: %w", err)
	}
	return role, nil
}

func (r *postgresRepository) AddMember(ctx context.Context, orgID, userID uuid.UUID, role authz.Role) error {
	_, err := database.Conn(ctx, r.db).Exec(ctx,
		`INSERT INTO organization_members (org_id, user_id, role) VALUES ($1, $2, $3)`, orgID, userID, role)
	if err != nil {
		return fmt.Errorf("insert organization member: %w", err)
	}
	return nil
}

const memberSelect = `
	SELECT u.id, u.email, u.first_name, u.last_name, u.avatar_url, m.role, m.joined_at
	FROM organization_members m
	JOIN users u ON u.id = m.user_id AND u.deleted_at IS NULL
`

func scanMember(row interface{ Scan(...any) error }, m *Member) error {
	return row.Scan(&m.UserID, &m.Email, &m.FirstName, &m.LastName, &m.AvatarURL, &m.Role, &m.JoinedAt)
}

func (r *postgresRepository) GetMember(ctx context.Context, orgID, userID uuid.UUID) (*Member, error) {
	query := memberSelect + ` WHERE m.org_id = $1 AND m.user_id = $2`
	var m Member
	if err := scanMember(database.Conn(ctx, r.db).QueryRow(ctx, query, orgID, userID), &m); err != nil {
		if database.IsNoRows(err) {
			return nil, ErrMemberNotFound
		}
		return nil, fmt.Errorf("select member: %w", err)
	}
	return &m, nil
}

func (r *postgresRepository) ListMembers(ctx context.Context, orgID uuid.UUID, p pagination.Params) ([]Member, error) {
	args := []any{orgID, p.Limit + 1}
	cursorClause := ""
	if p.After != nil {
		cursorClause = `AND (m.joined_at, m.user_id) > ($3, $4)`
		args = append(args, p.After.Time, p.After.ID)
	}
	query := memberSelect + ` WHERE m.org_id = $1 ` + cursorClause + `
		ORDER BY m.joined_at ASC, m.user_id ASC
		LIMIT $2`

	rows, err := database.Conn(ctx, r.db).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()

	var out []Member
	for rows.Next() {
		var m Member
		if err := scanMember(rows, &m); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *postgresRepository) CountOwners(ctx context.Context, orgID uuid.UUID) (int, error) {
	var n int
	err := database.Conn(ctx, r.db).QueryRow(ctx,
		`SELECT count(*) FROM organization_members WHERE org_id = $1 AND role = 'owner'`, orgID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count owners: %w", err)
	}
	return n, nil
}

func (r *postgresRepository) UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role authz.Role) error {
	tag, err := database.Conn(ctx, r.db).Exec(ctx,
		`UPDATE organization_members SET role = $3, updated_at = $4 WHERE org_id = $1 AND user_id = $2`,
		orgID, userID, role, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("update member role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMemberNotFound
	}
	return nil
}

func (r *postgresRepository) RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error {
	tag, err := database.Conn(ctx, r.db).Exec(ctx,
		`DELETE FROM organization_members WHERE org_id = $1 AND user_id = $2`, orgID, userID)
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMemberNotFound
	}
	return nil
}
