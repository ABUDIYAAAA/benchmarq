package organizations

import (
	"net/http"

	"github.com/ABUDIYAAAA/benchmarq/internal/authz"
	"github.com/ABUDIYAAAA/benchmarq/internal/middleware"
	"github.com/ABUDIYAAAA/benchmarq/pkg/apperr"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/pagination"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/request"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/response"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/validator"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes mounts routes that are not scoped to a single organization.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/", h.Create)
	r.Get("/", h.ListMine)
}

// RegisterOrgRoutes mounts routes under /orgs/{orgID}. The caller must have
// installed authz.RequireMembership on r.
func (h *Handler) RegisterOrgRoutes(r chi.Router) {
	r.With(authz.Require(authz.PermOrgView)).Get("/", h.Get)
	r.With(authz.Require(authz.PermOrgUpdate)).Patch("/", h.Update)
	r.With(authz.Require(authz.PermOrgDelete)).Delete("/", h.Delete)

	r.Route("/members", func(r chi.Router) {
		r.With(authz.Require(authz.PermMembersView)).Get("/", h.ListMembers)
		r.With(authz.Require(authz.PermMembersManage)).Patch("/{userID}", h.UpdateMemberRole)
		// Permission is checked in the service: members may always remove themselves.
		r.Delete("/{userID}", h.RemoveMember)
	})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	var in OrganizationInput
	if errs, err := validator.DecodeAndValidate(r, &in); err != nil {
		response.ValidationError(w, errs)
		return
	}

	org, err := h.service.Create(r.Context(), userID, in)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusCreated, "Organization created", org)
}

func (h *Handler) ListMine(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	p, err := pagination.FromRequest(r)
	if err != nil {
		response.Fail(w, err)
		return
	}

	page, err := h.service.ListMine(r.Context(), userID, p)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Organizations loaded", page)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	m, _ := authz.MembershipFrom(r.Context())
	org, err := h.service.Get(r.Context(), m)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Organization loaded", org)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	m, _ := authz.MembershipFrom(r.Context())
	patch, err := request.ReadJSONObject(w, r)
	if err != nil {
		response.Fail(w, err)
		return
	}

	org, err := h.service.Update(r.Context(), m, patch)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Organization updated", org)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	m, _ := authz.MembershipFrom(r.Context())
	if err := h.service.Delete(r.Context(), m); err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Organization deleted", nil)
}

func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	m, _ := authz.MembershipFrom(r.Context())
	p, err := pagination.FromRequest(r)
	if err != nil {
		response.Fail(w, err)
		return
	}

	page, err := h.service.ListMembers(r.Context(), m, p)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Members loaded", page)
}

func (h *Handler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	m, _ := authz.MembershipFrom(r.Context())
	targetID, err := request.URLParamUUID(r, "userID")
	if err != nil {
		response.Fail(w, err)
		return
	}
	var req UpdateMemberRoleRequest
	if errs, err := validator.DecodeAndValidate(r, &req); err != nil {
		response.Fail(w, apperr.Validation(errs))
		return
	}

	member, err := h.service.UpdateMemberRole(r.Context(), m, targetID, req.Role)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Member role updated", member)
}

func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	m, _ := authz.MembershipFrom(r.Context())
	targetID, err := request.URLParamUUID(r, "userID")
	if err != nil {
		response.Fail(w, err)
		return
	}

	if err := h.service.RemoveMember(r.Context(), m, targetID); err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Member removed", nil)
}
