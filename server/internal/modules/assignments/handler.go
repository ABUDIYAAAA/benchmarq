package assignments

import (
	"net/http"

	"github.com/ABUDIYAAAA/benchmarq/internal/authz"
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

// RegisterRoutes mounts assignment routes under
// /orgs/{orgID}/exams/{examID}/assignments.
func (h *Handler) RegisterRoutes(r chi.Router) {
	assign := authz.Require(authz.PermExamAssign)

	r.With(authz.Require(authz.PermExamView)).Get("/", h.List)
	r.With(assign).Post("/", h.Assign)
	r.With(assign).Delete("/{assignmentID}", h.Unassign)
}

func (h *Handler) Assign(w http.ResponseWriter, r *http.Request) {
	m, _ := authz.MembershipFrom(r.Context())
	examID, err := request.URLParamUUID(r, "examID")
	if err != nil {
		response.Fail(w, err)
		return
	}
	var req AssignRequest
	if errs, err := validator.DecodeAndValidate(r, &req); err != nil {
		response.Fail(w, apperr.Validation(errs))
		return
	}

	result, err := h.service.Assign(r.Context(), m, examID, req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Candidates assigned", result)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	m, _ := authz.MembershipFrom(r.Context())
	examID, err := request.URLParamUUID(r, "examID")
	if err != nil {
		response.Fail(w, err)
		return
	}
	p, err := pagination.FromRequest(r)
	if err != nil {
		response.Fail(w, err)
		return
	}

	page, err := h.service.List(r.Context(), m, examID, p)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Assignments loaded", page)
}

func (h *Handler) Unassign(w http.ResponseWriter, r *http.Request) {
	m, _ := authz.MembershipFrom(r.Context())
	examID, err := request.URLParamUUID(r, "examID")
	if err != nil {
		response.Fail(w, err)
		return
	}
	assignmentID, err := request.URLParamUUID(r, "assignmentID")
	if err != nil {
		response.Fail(w, err)
		return
	}

	if err := h.service.Unassign(r.Context(), m, examID, assignmentID); err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Assignment removed", nil)
}
