package authz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ABUDIYAAAA/benchmarq/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestRolePermissions(t *testing.T) {
	cases := []struct {
		role Role
		perm Permission
		want bool
	}{
		{RoleOwner, PermOrgDelete, true},
		{RoleAdmin, PermOrgDelete, false},
		{RoleAdmin, PermMembersManage, true},
		{RoleInstructor, PermExamAuthor | PermExamPublish, true},
		{RoleInstructor, PermExamDelete, false},
		{RoleProctor, PermExamView, false},
		{RoleCandidate, PermExamView, false},
		{RoleCandidate, PermOrgView, true},
		{Role("ghost"), PermOrgView, false},
	}
	for _, tc := range cases {
		if got := tc.role.Can(tc.perm); got != tc.want {
			t.Errorf("%s.Can(%b) = %v, want %v", tc.role, tc.perm, got, tc.want)
		}
	}
}

func TestOutranks(t *testing.T) {
	if !RoleOwner.Outranks(RoleAdmin) || RoleAdmin.Outranks(RoleAdmin) || RoleProctor.Outranks(RoleInstructor) {
		t.Fatal("unexpected role ordering")
	}
}

type staticResolver map[uuid.UUID]Role

func (s staticResolver) ResolveRole(_ context.Context, orgID, _ uuid.UUID) (Role, error) {
	if role, ok := s[orgID]; ok {
		return role, nil
	}
	return "", ErrNotMember
}

func TestMiddlewareChain(t *testing.T) {
	orgID, otherOrg := uuid.New(), uuid.New()
	resolver := staticResolver{orgID: RoleProctor}

	r := chi.NewRouter()
	r.Route("/orgs/{orgID}", func(r chi.Router) {
		r.Use(RequireMembership(resolver))
		r.Get("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
		r.With(Require(PermExamAuthor)).Post("/exams", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusCreated) })
	})

	do := func(method, path string) int {
		req := httptest.NewRequest(method, path, nil)
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDContextKey, uuid.New()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	if code := do("GET", "/orgs/"+orgID.String()+"/"); code != http.StatusOK {
		t.Errorf("member view: got %d", code)
	}
	if code := do("POST", "/orgs/"+orgID.String()+"/exams"); code != http.StatusForbidden {
		t.Errorf("proctor authoring: got %d, want 403", code)
	}
	if code := do("GET", "/orgs/"+otherOrg.String()+"/"); code != http.StatusNotFound {
		t.Errorf("non-member: got %d, want 404", code)
	}
	if code := do("GET", "/orgs/not-a-uuid/"); code != http.StatusUnprocessableEntity {
		t.Errorf("bad uuid: got %d, want 422", code)
	}
}
