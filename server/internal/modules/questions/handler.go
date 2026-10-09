package questions

import (
	"net/http"

	"github.com/ABUDIYAAAA/benchmarq/internal/authz"
	"github.com/ABUDIYAAAA/benchmarq/pkg/apperr"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/request"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/response"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/validator"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes mounts question routes under
// /orgs/{orgID}/exams/{examID}/sections/{sectionID}/questions.
func (h *Handler) RegisterRoutes(r chi.Router) {
	view := authz.Require(authz.PermExamView)
	author := authz.Require(authz.PermExamAuthor)

	r.With(view).Get("/", h.List)
	r.With(author).Post("/", h.Create)
	r.With(author).Put("/order", h.Reorder)
	r.With(view).Get("/{questionID}", h.Get)
	r.With(author).Put("/{questionID}", h.Replace)
	r.With(author).Delete("/{questionID}", h.Delete)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	scope, ok := sectionScope(w, r)
	if !ok {
		return
	}
	qs, err := h.service.List(r.Context(), scope)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Questions loaded", qs)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	scope, questionID, ok := questionScope(w, r)
	if !ok {
		return
	}
	q, err := h.service.Get(r.Context(), scope, questionID)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Question loaded", q)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	scope, ok := sectionScope(w, r)
	if !ok {
		return
	}
	var req CreateQuestionRequest
	if errs, err := validator.DecodeAndValidate(r, &req); err != nil {
		response.Fail(w, apperr.Validation(errs))
		return
	}
	q, err := h.service.Create(r.Context(), scope, req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusCreated, "Question created", q)
}

func (h *Handler) Replace(w http.ResponseWriter, r *http.Request) {
	scope, questionID, ok := questionScope(w, r)
	if !ok {
		return
	}
	var in QuestionInput
	if errs, err := validator.DecodeAndValidate(r, &in); err != nil {
		response.Fail(w, apperr.Validation(errs))
		return
	}
	q, err := h.service.Replace(r.Context(), scope, questionID, in)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Question updated", q)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	scope, questionID, ok := questionScope(w, r)
	if !ok {
		return
	}
	if err := h.service.Delete(r.Context(), scope, questionID); err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Question deleted", nil)
}

func (h *Handler) Reorder(w http.ResponseWriter, r *http.Request) {
	scope, ok := sectionScope(w, r)
	if !ok {
		return
	}
	var req ReorderRequest
	if errs, err := validator.DecodeAndValidate(r, &req); err != nil {
		response.Fail(w, apperr.Validation(errs))
		return
	}
	qs, err := h.service.Reorder(r.Context(), scope, req.QuestionIDs)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Questions reordered", qs)
}

func sectionScope(w http.ResponseWriter, r *http.Request) (Scope, bool) {
	m, _ := authz.MembershipFrom(r.Context())
	examID, err := request.URLParamUUID(r, "examID")
	if err != nil {
		response.Fail(w, err)
		return Scope{}, false
	}
	sectionID, err := request.URLParamUUID(r, "sectionID")
	if err != nil {
		response.Fail(w, err)
		return Scope{}, false
	}
	return ScopeFrom(m, examID, sectionID), true
}

func questionScope(w http.ResponseWriter, r *http.Request) (Scope, uuid.UUID, bool) {
	scope, ok := sectionScope(w, r)
	if !ok {
		return scope, uuid.Nil, false
	}
	questionID, err := request.URLParamUUID(r, "questionID")
	if err != nil {
		response.Fail(w, err)
		return scope, uuid.Nil, false
	}
	return scope, questionID, true
}
