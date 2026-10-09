package exams

import (
	"net/http"
	"strings"

	"github.com/ABUDIYAAAA/benchmarq/internal/authz"
	"github.com/ABUDIYAAAA/benchmarq/pkg/apperr"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/pagination"
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

// RegisterRoutes mounts exam routes under /orgs/{orgID}/exams. The caller
// must have installed authz.RequireMembership. questionRoutes, if non-nil, is
// mounted at /{examID}/sections/{sectionID}/questions.
func (h *Handler) RegisterRoutes(r chi.Router, questionRoutes func(chi.Router)) {
	view := authz.Require(authz.PermExamView)
	author := authz.Require(authz.PermExamAuthor)
	publish := authz.Require(authz.PermExamPublish)

	r.With(view).Get("/", h.ListExams)
	r.With(author).Post("/", h.CreateExam)

	r.Route("/{examID}", func(r chi.Router) {
		r.With(view).Get("/", h.GetExam)
		r.With(author).Patch("/", h.UpdateExam)
		r.With(authz.Require(authz.PermExamDelete)).Delete("/", h.DeleteExam)

		r.With(view).Get("/config", h.GetConfig)
		r.With(author).Patch("/config", h.UpdateConfig)

		r.With(view).Get("/readiness", h.Readiness)
		r.With(publish).Post("/publish", h.transition(TransitionPublish, "Exam published"))
		r.With(publish).Post("/unpublish", h.transition(TransitionUnpublish, "Exam reverted to draft"))
		r.With(publish).Post("/archive", h.transition(TransitionArchive, "Exam archived"))
		r.With(publish).Post("/restore", h.transition(TransitionRestore, "Exam restored to draft"))

		r.Route("/sections", func(r chi.Router) {
			r.With(view).Get("/", h.ListSections)
			r.With(author).Post("/", h.CreateSection)
			r.With(author).Put("/order", h.ReorderSections)
			r.With(view).Get("/{sectionID}", h.GetSection)
			r.With(author).Patch("/{sectionID}", h.UpdateSection)
			r.With(author).Delete("/{sectionID}", h.DeleteSection)
			if questionRoutes != nil {
				r.Route("/{sectionID}/questions", questionRoutes)
			}
		})
	})
}

func (h *Handler) CreateExam(w http.ResponseWriter, r *http.Request) {
	m, _ := authz.MembershipFrom(r.Context())
	var req CreateExamRequest
	if errs, err := validator.DecodeAndValidate(r, &req); err != nil {
		response.Fail(w, apperr.Validation(errs))
		return
	}

	detail, err := h.service.CreateExam(r.Context(), m, req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusCreated, "Exam created", detail)
}

func (h *Handler) ListExams(w http.ResponseWriter, r *http.Request) {
	m, _ := authz.MembershipFrom(r.Context())
	p, err := pagination.FromRequest(r)
	if err != nil {
		response.Fail(w, err)
		return
	}

	q := r.URL.Query()
	filter := ExamFilter{Search: strings.TrimSpace(q.Get("q"))}
	if raw := q.Get("status"); raw != "" {
		status := ExamStatus(raw)
		switch status {
		case StatusDraft, StatusPublished, StatusArchived, StatusConcluded:
			filter.Status = &status
		default:
			response.Fail(w, apperr.Validation(map[string]string{"status": "status must be one of [draft published archived concluded]"}))
			return
		}
	}

	page, err := h.service.ListExams(r.Context(), m, filter, p)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Exams loaded", page)
}

func (h *Handler) GetExam(w http.ResponseWriter, r *http.Request) {
	m, examID, ok := examScope(w, r)
	if !ok {
		return
	}
	detail, err := h.service.GetExam(r.Context(), m, examID)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Exam loaded", detail)
}

func (h *Handler) UpdateExam(w http.ResponseWriter, r *http.Request) {
	m, examID, ok := examScope(w, r)
	if !ok {
		return
	}
	patch, err := request.ReadJSONObject(w, r)
	if err != nil {
		response.Fail(w, err)
		return
	}
	exam, err := h.service.UpdateExam(r.Context(), m, examID, patch)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Exam updated", exam)
}

func (h *Handler) DeleteExam(w http.ResponseWriter, r *http.Request) {
	m, examID, ok := examScope(w, r)
	if !ok {
		return
	}
	if err := h.service.DeleteExam(r.Context(), m, examID); err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Exam deleted", nil)
}

func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	m, examID, ok := examScope(w, r)
	if !ok {
		return
	}
	cfg, err := h.service.GetConfig(r.Context(), m, examID)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Exam configuration loaded", cfg)
}

func (h *Handler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	m, examID, ok := examScope(w, r)
	if !ok {
		return
	}
	patch, err := request.ReadJSONObject(w, r)
	if err != nil {
		response.Fail(w, err)
		return
	}
	cfg, err := h.service.UpdateConfig(r.Context(), m, examID, patch)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Exam configuration updated", cfg)
}

func (h *Handler) Readiness(w http.ResponseWriter, r *http.Request) {
	m, examID, ok := examScope(w, r)
	if !ok {
		return
	}
	readiness, err := h.service.Readiness(r.Context(), m, examID)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Readiness assessed", readiness)
}

func (h *Handler) transition(action Transition, message string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m, examID, ok := examScope(w, r)
		if !ok {
			return
		}
		exam, err := h.service.Transition(r.Context(), m, examID, action)
		if err != nil {
			response.Fail(w, err)
			return
		}
		response.Success(w, http.StatusOK, message, exam)
	}
}

func (h *Handler) ListSections(w http.ResponseWriter, r *http.Request) {
	m, examID, ok := examScope(w, r)
	if !ok {
		return
	}
	sections, err := h.service.ListSections(r.Context(), m, examID)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Sections loaded", sections)
}

func (h *Handler) GetSection(w http.ResponseWriter, r *http.Request) {
	m, examID, sectionID, ok := sectionScope(w, r)
	if !ok {
		return
	}
	section, err := h.service.GetSection(r.Context(), m, examID, sectionID)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Section loaded", section)
}

func (h *Handler) CreateSection(w http.ResponseWriter, r *http.Request) {
	m, examID, ok := examScope(w, r)
	if !ok {
		return
	}
	var req CreateSectionRequest
	if errs, err := validator.DecodeAndValidate(r, &req); err != nil {
		response.Fail(w, apperr.Validation(errs))
		return
	}
	section, err := h.service.CreateSection(r.Context(), m, examID, req)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusCreated, "Section created", section)
}

func (h *Handler) UpdateSection(w http.ResponseWriter, r *http.Request) {
	m, examID, sectionID, ok := sectionScope(w, r)
	if !ok {
		return
	}
	patch, err := request.ReadJSONObject(w, r)
	if err != nil {
		response.Fail(w, err)
		return
	}
	section, err := h.service.UpdateSection(r.Context(), m, examID, sectionID, patch)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Section updated", section)
}

func (h *Handler) DeleteSection(w http.ResponseWriter, r *http.Request) {
	m, examID, sectionID, ok := sectionScope(w, r)
	if !ok {
		return
	}
	if err := h.service.DeleteSection(r.Context(), m, examID, sectionID); err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Section deleted", nil)
}

func (h *Handler) ReorderSections(w http.ResponseWriter, r *http.Request) {
	m, examID, ok := examScope(w, r)
	if !ok {
		return
	}
	var req ReorderRequest
	if errs, err := validator.DecodeAndValidate(r, &req); err != nil {
		response.Fail(w, apperr.Validation(errs))
		return
	}
	sections, err := h.service.ReorderSections(r.Context(), m, examID, req.SectionIDs)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Sections reordered", sections)
}

func examScope(w http.ResponseWriter, r *http.Request) (authz.Membership, uuid.UUID, bool) {
	m, _ := authz.MembershipFrom(r.Context())
	examID, err := request.URLParamUUID(r, "examID")
	if err != nil {
		response.Fail(w, err)
		return m, uuid.Nil, false
	}
	return m, examID, true
}

func sectionScope(w http.ResponseWriter, r *http.Request) (authz.Membership, uuid.UUID, uuid.UUID, bool) {
	m, examID, ok := examScope(w, r)
	if !ok {
		return m, uuid.Nil, uuid.Nil, false
	}
	sectionID, err := request.URLParamUUID(r, "sectionID")
	if err != nil {
		response.Fail(w, err)
		return m, uuid.Nil, uuid.Nil, false
	}
	return m, examID, sectionID, true
}
