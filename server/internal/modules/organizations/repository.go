package organizations

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	ErrUserNotFound         = errors.New("user not found")
	ErrInvitationNotFound   = errors.New("invitation not found")
	ErrInvitationPending    = errors.New("a pending invitation already exists for this email")
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
	IsMemberEmail(ctx context.Context, orgID uuid.UUID, email string) (bool, error)
	GetUserEmail(ctx context.Context, userID uuid.UUID) (string, error)

	// CreateInvitation inserts a pending invitation. A pending invitation for
	// the same email that has already expired is marked expired first.
	CreateInvitation(ctx context.Context, inv *Invitation) error
	GetInvitation(ctx context.Context, orgID, invitationID uuid.UUID) (*Invitation, error)
	// LockInvitationByToken loads an invitation by token hash with a row lock.
	LockInvitationByToken(ctx context.Context, tokenHash string) (*Invitation, error)
	ListInvitations(ctx context.Context, orgID uuid.UUID, f InvitationFilter, p pagination.Params) ([]Invitation, error)
	// ReissueInvitation replaces the token and expiry of a pending invitation.
	ReissueInvitation(ctx context.Context, inv *Invitation) error
	SetInvitationStatus(ctx context.Context, invitationID uuid.UUID, status InvitationStatus) error
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

func (r *postgresRepository) IsMemberEmail(ctx context.Context, orgID uuid.UUID, email string) (bool, error) {
	var exists bool
	err := database.Conn(ctx, r.db).QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM organization_members m
			JOIN users u ON u.id = m.user_id AND u.deleted_at IS NULL
			WHERE m.org_id = $1 AND u.email = $2
		)`, orgID, email).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check member email: %w", err)
	}
	return exists, nil
}

func (r *postgresRepository) GetUserEmail(ctx context.Context, userID uuid.UUID) (string, error) {
	var email string
	err := database.Conn(ctx, r.db).QueryRow(ctx,
		`SELECT email FROM users WHERE id = $1 AND deleted_at IS NULL`, userID).Scan(&email)
	if err != nil {
		if database.IsNoRows(err) {
			return "", ErrUserNotFound
		}
		return "", fmt.Errorf("select user email: %w", err)
	}
	return email, nil
}

// invitationSelect reports pending invitations past their expiry as expired.
const invitationSelect = `
	SELECT id, org_id, email,
	       role,
	       CASE WHEN status = 'pending' AND expires_at <= now() THEN 'expired' ELSE status::text END,
	       invited_by_user_id, expires_at, accepted_at, revoked_at, created_at, updated_at, token_hash
	FROM organization_invitations
`

func scanInvitation(row interface{ Scan(...any) error }, inv *Invitation) error {
	return row.Scan(&inv.ID, &inv.OrgID, &inv.Email, &inv.Role, &inv.Status, &inv.InvitedByUserID,
		&inv.ExpiresAt, &inv.AcceptedAt, &inv.RevokedAt, &inv.CreatedAt, &inv.UpdatedAt, &inv.TokenHash)
}

func (r *postgresRepository) CreateInvitation(ctx context.Context, inv *Invitation) error {
	if inv.ID == uuid.Nil {
		inv.ID = uuid.New()
	}
	conn := database.Conn(ctx, r.db)
	_, err := conn.Exec(ctx, `
		UPDATE organization_invitations
		SET status = 'expired', updated_at = now()
		WHERE org_id = $1 AND email = $2 AND status = 'pending' AND expires_at <= now()
	`, inv.OrgID, inv.Email)
	if err != nil {
		return fmt.Errorf("expire stale invitation: %w", err)
	}

	err = conn.QueryRow(ctx, `
		INSERT INTO organization_invitations (id, org_id, email, role, token_hash, invited_by_user_id, status, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7)
		RETURNING status, created_at, updated_at
	`, inv.ID, inv.OrgID, inv.Email, inv.Role, inv.TokenHash, inv.InvitedByUserID, inv.ExpiresAt,
	).Scan(&inv.Status, &inv.CreatedAt, &inv.UpdatedAt)
	if err != nil {
		if database.IsUniqueViolation(err, "organization_invitations_one_pending") {
			return ErrInvitationPending
		}
		return fmt.Errorf("insert invitation: %w", err)
	}
	return nil
}

func (r *postgresRepository) GetInvitation(ctx context.Context, orgID, invitationID uuid.UUID) (*Invitation, error) {
	var inv Invitation
	err := scanInvitation(database.Conn(ctx, r.db).QueryRow(ctx,
		invitationSelect+` WHERE org_id = $1 AND id = $2`, orgID, invitationID), &inv)
	if err != nil {
		if database.IsNoRows(err) {
			return nil, ErrInvitationNotFound
		}
		return nil, fmt.Errorf("select invitation: %w", err)
	}
	return &inv, nil
}

func (r *postgresRepository) LockInvitationByToken(ctx context.Context, tokenHash string) (*Invitation, error) {
	var inv Invitation
	err := scanInvitation(database.Conn(ctx, r.db).QueryRow(ctx,
		invitationSelect+` WHERE token_hash = $1 FOR UPDATE`, tokenHash), &inv)
	if err != nil {
		if database.IsNoRows(err) {
			return nil, ErrInvitationNotFound
		}
		return nil, fmt.Errorf("select invitation by token: %w", err)
	}
	return &inv, nil
}

func (r *postgresRepository) ListInvitations(ctx context.Context, orgID uuid.UUID, f InvitationFilter, p pagination.Params) ([]Invitation, error) {
	args := []any{orgID, p.Limit + 1}
	where := []string{"org_id = $1"}
	switch f.Status {
	case InvitationPending:
		where = append(where, "status = 'pending' AND expires_at > now()")
	case InvitationExpired:
		where = append(where, "(status = 'expired' OR (status = 'pending' AND expires_at <= now()))")
	case InvitationAccepted, InvitationRevoked:
		args = append(args, f.Status)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	if p.After != nil {
		args = append(args, p.After.Time, p.After.ID)
		where = append(where, fmt.Sprintf("(created_at, id) < ($%d, $%d)", len(args)-1, len(args)))
	}
	query := invitationSelect + ` WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY created_at DESC, id DESC
		LIMIT $2`

	rows, err := database.Conn(ctx, r.db).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	defer rows.Close()

	var out []Invitation
	for rows.Next() {
		var inv Invitation
		if err := scanInvitation(rows, &inv); err != nil {
			return nil, fmt.Errorf("scan invitation: %w", err)
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func (r *postgresRepository) ReissueInvitation(ctx context.Context, inv *Invitation) error {
	err := database.Conn(ctx, r.db).QueryRow(ctx, `
		UPDATE organization_invitations
		SET token_hash = $2, expires_at = $3, invited_by_user_id = $4, updated_at = now()
		WHERE id = $1 AND status = 'pending'
		RETURNING updated_at
	`, inv.ID, inv.TokenHash, inv.ExpiresAt, inv.InvitedByUserID).Scan(&inv.UpdatedAt)
	if err != nil {
		if database.IsNoRows(err) {
			return ErrInvitationNotFound
		}
		return fmt.Errorf("reissue invitation: %w", err)
	}
	inv.Status = InvitationPending
	return nil
}

func (r *postgresRepository) SetInvitationStatus(ctx context.Context, invitationID uuid.UUID, status InvitationStatus) error {
	_, err := database.Conn(ctx, r.db).Exec(ctx, `
		UPDATE organization_invitations
		SET status = $2::invitation_status,
		    accepted_at = CASE WHEN $2::invitation_status = 'accepted' THEN now() ELSE accepted_at END,
		    revoked_at = CASE WHEN $2::invitation_status = 'revoked' THEN now() ELSE revoked_at END,
		    updated_at = now()
		WHERE id = $1
	`, invitationID, status)
	if err != nil {
		return fmt.Errorf("update invitation status: %w", err)
	}
	return nil
}
