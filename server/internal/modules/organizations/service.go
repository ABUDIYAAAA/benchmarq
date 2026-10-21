package organizations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ABUDIYAAAA/benchmarq/internal/authz"
	"github.com/ABUDIYAAAA/benchmarq/internal/database"
	"github.com/ABUDIYAAAA/benchmarq/internal/mailer"
	"github.com/ABUDIYAAAA/benchmarq/pkg/apperr"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/pagination"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/request"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/token"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/validator"
	"github.com/google/uuid"
)

const maxSlugAttempts = 5

var (
	errOrgNotFound    = apperr.NotFound("Organization not found")
	errMemberNotFound = apperr.NotFound("Member not found")
	errSlugTaken      = apperr.Conflict("Organization slug is already taken")
	errLastOwner      = apperr.Conflict("An organization must keep at least one owner; transfer ownership first")
	errSelfRoleChange = apperr.Forbidden("You cannot change your own role")
	errOutranked      = apperr.Forbidden("You can only manage members with a lower role than your own")

	errInvitationNotFound   = apperr.NotFound("Invitation not found")
	errInvitationPending    = apperr.Conflict("A pending invitation already exists for this email; resend it instead")
	errAlreadyMember        = apperr.Conflict("This person is already a member of the organization")
	errInvitationNotPending = apperr.Conflict("Only pending invitations can be changed")
	errInvitationInvalid    = apperr.NotFound("Invitation not found or no longer valid")
	errInvitationExpired    = apperr.Conflict("This invitation has expired; ask for a new one")
	errInvitationWrongEmail = apperr.Forbidden("This invitation was sent to a different email address")
)

// InviteOptions configures organization invitations.
type InviteOptions struct {
	Mailer      mailer.Mailer // optional; invitations are created without email when nil
	FrontendURL string
	ExpiryDays  int
}

type Service interface {
	Create(ctx context.Context, userID uuid.UUID, in OrganizationInput) (*OrganizationResponse, error)
	ListMine(ctx context.Context, userID uuid.UUID, p pagination.Params) (pagination.Page[*OrganizationResponse], error)
	Get(ctx context.Context, m authz.Membership) (*OrganizationResponse, error)
	Update(ctx context.Context, m authz.Membership, patch json.RawMessage) (*OrganizationResponse, error)
	Delete(ctx context.Context, m authz.Membership) error

	ListMembers(ctx context.Context, m authz.Membership, p pagination.Params) (pagination.Page[Member], error)
	UpdateMemberRole(ctx context.Context, m authz.Membership, targetUserID uuid.UUID, role authz.Role) (*Member, error)
	RemoveMember(ctx context.Context, m authz.Membership, targetUserID uuid.UUID) error

	CreateInvitation(ctx context.Context, m authz.Membership, req CreateInvitationRequest) (*Invitation, error)
	ListInvitations(ctx context.Context, m authz.Membership, f InvitationFilter, p pagination.Params) (pagination.Page[Invitation], error)
	ResendInvitation(ctx context.Context, m authz.Membership, invitationID uuid.UUID) (*Invitation, error)
	RevokeInvitation(ctx context.Context, m authz.Membership, invitationID uuid.UUID) error
	AcceptInvitation(ctx context.Context, userID uuid.UUID, rawToken string) (*OrganizationResponse, error)
}

type service struct {
	repo   Repository
	tx     database.Transactor
	logger *slog.Logger
	invite InviteOptions
	now    func() time.Time
}

func NewService(repo Repository, tx database.Transactor, logger *slog.Logger, invite InviteOptions) Service {
	if invite.ExpiryDays <= 0 {
		invite.ExpiryDays = 7
	}
	return &service{repo: repo, tx: tx, logger: logger, invite: invite, now: func() time.Time { return time.Now().UTC() }}
}

func (s *service) Create(ctx context.Context, userID uuid.UUID, in OrganizationInput) (*OrganizationResponse, error) {
	org := &Organization{
		Name:            in.Name,
		Slug:            in.Slug,
		LogoURL:         in.LogoURL,
		Website:         in.Website,
		Branding:        in.Branding,
		CreatedByUserID: &userID,
	}
	explicitSlug := in.Slug != ""
	if !explicitSlug {
		org.Slug = Slugify(in.Name)
	}

	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.insertWithAvailableSlug(ctx, org, explicitSlug); err != nil {
			return err
		}
		return s.repo.AddMember(ctx, org.ID, userID, authz.RoleOwner)
	})
	if err != nil {
		return nil, s.mapErr(err, "create organization")
	}
	return toResponse(org, authz.RoleOwner), nil
}

// insertWithAvailableSlug inserts org, appending a random suffix to generated
// slugs on collision. Slugs the user chose explicitly are never altered.
func (s *service) insertWithAvailableSlug(ctx context.Context, org *Organization, explicit bool) error {
	base := org.Slug
	for attempt := 0; attempt < maxSlugAttempts; attempt++ {
		err := s.repo.Create(ctx, org)
		if !errors.Is(err, ErrSlugTaken) || explicit {
			return err
		}
		org.Slug = withRandomSuffix(base)
	}
	return ErrSlugTaken
}

func (s *service) ListMine(ctx context.Context, userID uuid.UUID, p pagination.Params) (pagination.Page[*OrganizationResponse], error) {
	rows, err := s.repo.ListForUser(ctx, userID, p)
	if err != nil {
		return pagination.Page[*OrganizationResponse]{}, s.mapErr(err, "list organizations")
	}
	page := pagination.NewPage(rows, p.Limit, func(m Membership) pagination.Cursor {
		return pagination.Cursor{Time: m.JoinedAt, ID: m.Organization.ID}
	})
	out := pagination.Page[*OrganizationResponse]{Items: make([]*OrganizationResponse, 0, len(page.Items)), NextCursor: page.NextCursor}
	for i := range page.Items {
		out.Items = append(out.Items, toResponse(&page.Items[i].Organization, page.Items[i].Role))
	}
	return out, nil
}

func (s *service) Get(ctx context.Context, m authz.Membership) (*OrganizationResponse, error) {
	org, err := s.repo.GetByID(ctx, m.OrgID)
	if err != nil {
		return nil, s.mapErr(err, "get organization")
	}
	return toResponse(org, m.Role), nil
}

func (s *service) Update(ctx context.Context, m authz.Membership, patch json.RawMessage) (*OrganizationResponse, error) {
	var org *Organization
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if org, err = s.repo.LockByID(ctx, m.OrgID); err != nil {
			return err
		}

		in := inputFrom(org)
		if err := request.ApplyMergePatch(&in, patch); err != nil {
			return err
		}
		if fields, ok := validator.ValidateStruct(in); !ok {
			return apperr.Validation(fields)
		}

		org.Name, org.Slug, org.LogoURL, org.Website, org.Branding = in.Name, in.Slug, in.LogoURL, in.Website, in.Branding
		if org.Slug == "" {
			org.Slug = Slugify(org.Name)
		}
		return s.repo.Update(ctx, org)
	})
	if err != nil {
		return nil, s.mapErr(err, "update organization")
	}
	return toResponse(org, m.Role), nil
}

func (s *service) Delete(ctx context.Context, m authz.Membership) error {
	if err := s.repo.SoftDelete(ctx, m.OrgID); err != nil {
		return s.mapErr(err, "delete organization")
	}
	return nil
}

func (s *service) ListMembers(ctx context.Context, m authz.Membership, p pagination.Params) (pagination.Page[Member], error) {
	rows, err := s.repo.ListMembers(ctx, m.OrgID, p)
	if err != nil {
		return pagination.Page[Member]{}, s.mapErr(err, "list members")
	}
	return pagination.NewPage(rows, p.Limit, func(mem Member) pagination.Cursor {
		return pagination.Cursor{Time: mem.JoinedAt, ID: mem.UserID}
	}), nil
}

// UpdateMemberRole enforces the role hierarchy:
//   - nobody changes their own role (prevents accidental self-lockout);
//   - non-owners may only manage members below them and grant roles below them;
//   - owners may manage anyone, including other owners;
//   - the organization always keeps at least one owner.
//
// The organization row is locked so concurrent demotions cannot race past the
// last-owner check.
func (s *service) UpdateMemberRole(ctx context.Context, m authz.Membership, targetUserID uuid.UUID, role authz.Role) (*Member, error) {
	if targetUserID == m.UserID {
		return nil, errSelfRoleChange
	}

	var target *Member
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := s.repo.LockByID(ctx, m.OrgID); err != nil {
			return err
		}
		var err error
		if target, err = s.repo.GetMember(ctx, m.OrgID, targetUserID); err != nil {
			return err
		}
		if target.Role == role {
			return nil
		}
		if m.Role != authz.RoleOwner && (!m.Role.Outranks(target.Role) || !m.Role.Outranks(role)) {
			return errOutranked
		}
		if target.Role == authz.RoleOwner {
			if err := s.ensureAnotherOwner(ctx, m.OrgID); err != nil {
				return err
			}
		}
		if err := s.repo.UpdateMemberRole(ctx, m.OrgID, targetUserID, role); err != nil {
			return err
		}
		target.Role = role
		return nil
	})
	if err != nil {
		return nil, s.mapErr(err, "update member role")
	}
	return target, nil
}

// RemoveMember removes a member. Any member may remove themselves (leave);
// removing someone else requires PermMembersManage and a higher role.
func (s *service) RemoveMember(ctx context.Context, m authz.Membership, targetUserID uuid.UUID) error {
	leaving := targetUserID == m.UserID
	if !leaving && !m.Can(authz.PermMembersManage) {
		return apperr.Forbidden("You do not have permission to perform this action")
	}

	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := s.repo.LockByID(ctx, m.OrgID); err != nil {
			return err
		}
		target, err := s.repo.GetMember(ctx, m.OrgID, targetUserID)
		if err != nil {
			return err
		}
		if !leaving && m.Role != authz.RoleOwner && !m.Role.Outranks(target.Role) {
			return errOutranked
		}
		if target.Role == authz.RoleOwner {
			if err := s.ensureAnotherOwner(ctx, m.OrgID); err != nil {
				return err
			}
		}
		return s.repo.RemoveMember(ctx, m.OrgID, targetUserID)
	})
	if err != nil {
		return s.mapErr(err, "remove member")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Invitations
// ---------------------------------------------------------------------------

// CreateInvitation invites an email address to the organization. Like role
// changes, non-owners may only invite to roles below their own.
func (s *service) CreateInvitation(ctx context.Context, m authz.Membership, req CreateInvitationRequest) (*Invitation, error) {
	if m.Role != authz.RoleOwner && !m.Role.Outranks(req.Role) {
		return nil, errOutranked
	}

	rawToken, tokenHash, err := token.GenerateOpaqueToken()
	if err != nil {
		return nil, apperr.Internal(err, "create invitation")
	}
	inv := &Invitation{
		OrgID:           m.OrgID,
		Email:           strings.ToLower(strings.TrimSpace(req.Email)),
		Role:            req.Role,
		InvitedByUserID: &m.UserID,
		ExpiresAt:       s.inviteExpiry(),
		TokenHash:       tokenHash,
	}

	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		member, err := s.repo.IsMemberEmail(ctx, m.OrgID, inv.Email)
		if err != nil {
			return err
		}
		if member {
			return errAlreadyMember
		}
		return s.repo.CreateInvitation(ctx, inv)
	})
	if err != nil {
		return nil, s.mapErr(err, "create invitation")
	}

	s.sendInvitation(ctx, m, inv, rawToken)
	return inv, nil
}

func (s *service) ListInvitations(ctx context.Context, m authz.Membership, f InvitationFilter, p pagination.Params) (pagination.Page[Invitation], error) {
	rows, err := s.repo.ListInvitations(ctx, m.OrgID, f, p)
	if err != nil {
		return pagination.Page[Invitation]{}, s.mapErr(err, "list invitations")
	}
	return pagination.NewPage(rows, p.Limit, func(inv Invitation) pagination.Cursor {
		return pagination.Cursor{Time: inv.CreatedAt, ID: inv.ID}
	}), nil
}

// ResendInvitation issues a new link for a pending (or lapsed) invitation and
// restarts its expiry. The previous link stops working.
func (s *service) ResendInvitation(ctx context.Context, m authz.Membership, invitationID uuid.UUID) (*Invitation, error) {
	rawToken, tokenHash, err := token.GenerateOpaqueToken()
	if err != nil {
		return nil, apperr.Internal(err, "resend invitation")
	}

	var inv *Invitation
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if inv, err = s.manageableInvitation(ctx, m, invitationID); err != nil {
			return err
		}
		inv.TokenHash = tokenHash
		inv.ExpiresAt = s.inviteExpiry()
		inv.InvitedByUserID = &m.UserID
		return s.repo.ReissueInvitation(ctx, inv)
	})
	if err != nil {
		return nil, s.mapErr(err, "resend invitation")
	}

	s.sendInvitation(ctx, m, inv, rawToken)
	return inv, nil
}

func (s *service) RevokeInvitation(ctx context.Context, m authz.Membership, invitationID uuid.UUID) error {
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := s.manageableInvitation(ctx, m, invitationID); err != nil {
			return err
		}
		return s.repo.SetInvitationStatus(ctx, invitationID, InvitationRevoked)
	})
	return s.mapErr(err, "revoke invitation")
}

// manageableInvitation loads an invitation the caller may resend or revoke:
// it must still be pending (or have lapsed) and be for a role below the
// caller's own, unless the caller is an owner.
func (s *service) manageableInvitation(ctx context.Context, m authz.Membership, invitationID uuid.UUID) (*Invitation, error) {
	inv, err := s.repo.GetInvitation(ctx, m.OrgID, invitationID)
	if err != nil {
		return nil, err
	}
	if inv.Status != InvitationPending && inv.Status != InvitationExpired {
		return nil, errInvitationNotPending
	}
	if m.Role != authz.RoleOwner && !m.Role.Outranks(inv.Role) {
		return nil, errOutranked
	}
	return inv, nil
}

// AcceptInvitation adds the signed-in user to the inviting organization. The
// user's account email must match the invited address.
func (s *service) AcceptInvitation(ctx context.Context, userID uuid.UUID, rawToken string) (*OrganizationResponse, error) {
	var (
		org  *Organization
		role authz.Role
	)
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		inv, err := s.repo.LockInvitationByToken(ctx, token.HashToken(rawToken))
		if err != nil {
			if errors.Is(err, ErrInvitationNotFound) {
				return errInvitationInvalid
			}
			return err
		}
		switch inv.Status {
		case InvitationPending:
		case InvitationExpired:
			return errInvitationExpired
		default:
			return errInvitationInvalid
		}

		email, err := s.repo.GetUserEmail(ctx, userID)
		if err != nil {
			return err
		}
		if !strings.EqualFold(email, inv.Email) {
			return errInvitationWrongEmail
		}

		if org, err = s.repo.LockByID(ctx, inv.OrgID); err != nil {
			if errors.Is(err, ErrOrganizationNotFound) {
				return errInvitationInvalid
			}
			return err
		}

		// Someone who joined by another route keeps their current role.
		role = inv.Role
		existing, err := s.repo.GetMember(ctx, inv.OrgID, userID)
		switch {
		case err == nil:
			role = existing.Role
		case errors.Is(err, ErrMemberNotFound):
			if err := s.repo.AddMember(ctx, inv.OrgID, userID, inv.Role); err != nil {
				return err
			}
		default:
			return err
		}
		return s.repo.SetInvitationStatus(ctx, inv.ID, InvitationAccepted)
	})
	if err != nil {
		return nil, s.mapErr(err, "accept invitation")
	}
	return toResponse(org, role), nil
}

func (s *service) inviteExpiry() time.Time {
	return s.now().Add(time.Duration(s.invite.ExpiryDays) * 24 * time.Hour)
}

// sendInvitation emails the invitation link. Failures are logged rather than
// returned: the invitation exists and can be resent.
func (s *service) sendInvitation(ctx context.Context, m authz.Membership, inv *Invitation, rawToken string) {
	if s.invite.Mailer == nil {
		return
	}
	org, err := s.repo.GetByID(ctx, m.OrgID)
	if err != nil {
		s.logger.Warn("organizations: load organization for invitation email", "invitation_id", inv.ID, "error", err)
		return
	}
	inviter := "A team member"
	if member, err := s.repo.GetMember(ctx, m.OrgID, m.UserID); err == nil {
		inviter = member.FirstName
		if member.LastName != nil && *member.LastName != "" {
			inviter += " " + *member.LastName
		}
	}

	acceptURL := fmt.Sprintf("%s/invitations/accept?token=%s", s.invite.FrontendURL, rawToken)
	if err := s.invite.Mailer.SendOrgInviteEmail(inv.Email, org.Name, inviter, string(inv.Role), acceptURL, s.invite.ExpiryDays); err != nil {
		s.logger.Warn("organizations: send invitation email", "invitation_id", inv.ID, "error", err)
	}
}

func (s *service) ensureAnotherOwner(ctx context.Context, orgID uuid.UUID) error {
	owners, err := s.repo.CountOwners(ctx, orgID)
	if err != nil {
		return err
	}
	if owners <= 1 {
		return errLastOwner
	}
	return nil
}

// mapErr converts repository errors into application errors.
func (s *service) mapErr(err error, op string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrOrganizationNotFound):
		return errOrgNotFound
	case errors.Is(err, ErrMemberNotFound):
		return errMemberNotFound
	case errors.Is(err, ErrSlugTaken):
		return errSlugTaken
	case errors.Is(err, ErrInvitationNotFound):
		return errInvitationNotFound
	case errors.Is(err, ErrInvitationPending):
		return errInvitationPending
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	s.logger.Error("organizations: "+op+" failed", "error", err)
	return apperr.Internal(err, op)
}
