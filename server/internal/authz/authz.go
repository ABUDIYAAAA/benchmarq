// Package authz implements organization-scoped role-based access control.
//
// Every org-scoped route first resolves the caller's membership (RequireMembership),
// then individual routes demand a Permission (Require). Roles map to permission
// sets through a single table, so changing what a role may do is one edit.
package authz

import (
	"context"
	"errors"
	"net/http"

	"github.com/ABUDIYAAAA/benchmarq/internal/middleware"
	"github.com/ABUDIYAAAA/benchmarq/pkg/apperr"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/request"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/response"
	"github.com/google/uuid"
)

type Role string

const (
	RoleOwner      Role = "owner"
	RoleAdmin      Role = "admin"
	RoleInstructor Role = "instructor"
	RoleProctor    Role = "proctor"
	RoleCandidate  Role = "candidate"
)

// rank orders roles by authority; higher outranks lower.
var rank = map[Role]int{
	RoleCandidate:  1,
	RoleProctor:    2,
	RoleInstructor: 3,
	RoleAdmin:      4,
	RoleOwner:      5,
}

func (r Role) Valid() bool { _, ok := rank[r]; return ok }

// Outranks reports whether r has strictly more authority than other.
func (r Role) Outranks(other Role) bool { return rank[r] > rank[other] }

type Permission uint32

const (
	PermOrgView Permission = 1 << iota
	PermOrgUpdate
	PermOrgDelete
	PermMembersView
	PermMembersManage
	PermExamView    // read exams including answer keys
	PermExamAuthor  // create and edit exams, sections and questions
	PermExamPublish // publish, unpublish, archive and restore
	PermExamDelete
)

var rolePermissions = map[Role]Permission{
	RoleOwner: PermOrgView | PermOrgUpdate | PermOrgDelete |
		PermMembersView | PermMembersManage |
		PermExamView | PermExamAuthor | PermExamPublish | PermExamDelete,
	RoleAdmin: PermOrgView | PermOrgUpdate |
		PermMembersView | PermMembersManage |
		PermExamView | PermExamAuthor | PermExamPublish | PermExamDelete,
	RoleInstructor: PermOrgView | PermMembersView |
		PermExamView | PermExamAuthor | PermExamPublish,
	RoleProctor:   PermOrgView | PermMembersView,
	RoleCandidate: PermOrgView,
}

// Can reports whether the role grants every permission in p.
func (r Role) Can(p Permission) bool {
	return rolePermissions[r]&p == p
}

// Membership is the caller's resolved relationship with the org in the URL.
type Membership struct {
	OrgID  uuid.UUID
	UserID uuid.UUID
	Role   Role
}

func (m Membership) Can(p Permission) bool { return m.Role.Can(p) }

// ErrNotMember is returned by a MembershipResolver when the user does not
// belong to the organization (or the organization does not exist).
var ErrNotMember = errors.New("not a member of this organization")

// MembershipResolver looks up a user's role within an organization.
type MembershipResolver interface {
	ResolveRole(ctx context.Context, orgID, userID uuid.UUID) (Role, error)
}

type membershipKey struct{}

func WithMembership(ctx context.Context, m Membership) context.Context {
	return context.WithValue(ctx, membershipKey{}, m)
}

func MembershipFrom(ctx context.Context) (Membership, bool) {
	m, ok := ctx.Value(membershipKey{}).(Membership)
	return m, ok
}

// RequireMembership resolves the caller's role in the {orgID} route parameter.
// Non-members get 404 rather than 403 so organization IDs cannot be probed.
func RequireMembership(resolver MembershipResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := middleware.GetUserID(r.Context())
			if !ok {
				response.Error(w, http.StatusUnauthorized, "Authentication required")
				return
			}
			orgID, err := request.URLParamUUID(r, "orgID")
			if err != nil {
				response.Fail(w, err)
				return
			}

			role, err := resolver.ResolveRole(r.Context(), orgID, userID)
			if err != nil {
				if errors.Is(err, ErrNotMember) {
					response.Fail(w, apperr.NotFound("Organization not found"))
					return
				}
				response.Fail(w, err)
				return
			}

			ctx := WithMembership(r.Context(), Membership{OrgID: orgID, UserID: userID, Role: role})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Require rejects callers whose role lacks permission p. It must run after
// RequireMembership.
func Require(p Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m, ok := MembershipFrom(r.Context())
			if !ok {
				response.Fail(w, apperr.NotFound("Organization not found"))
				return
			}
			if !m.Can(p) {
				response.Fail(w, apperr.Forbidden("You do not have permission to perform this action"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
