package attempts

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/ABUDIYAAAA/benchmarq/internal/authz"
	"github.com/ABUDIYAAAA/benchmarq/internal/middleware"
	"github.com/ABUDIYAAAA/benchmarq/pkg/apperr"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/pagination"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/request"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/response"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/validator"
	"github.com/go-chi/chi/v5"
)

// DeviceHeader carries the desktop app's install-time device ID.
const DeviceHeader = "X-Device-ID"

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// RegisterPortalRoutes mounts the web portal's candidate routes under
// /me/exams. The caller must have installed cookie authentication.
func (h *Handler) RegisterPortalRoutes(r chi.Router) {
	r.Get("/", h.ListMyExams)
	r.Post("/{examID}/launch", h.Launch)
}

// RegisterRuntimeRoutes mounts the desktop app's routes under /runtime.
// Everything except opening a session requires a session token and the
// matching device ID; browser cookies are never accepted here.
func (h *Handler) RegisterRuntimeRoutes(r chi.Router) {
	r.Post("/sessions", h.OpenSession)
	r.Group(func(r chi.Router) {
		r.Use(h.requireSession)
		r.Get("/state", h.State)
		r.Post("/start", h.Start)
		r.Post("/heartbeat", h.Heartbeat)
		r.Post("/sections/{sectionID}/enter", h.EnterSection)
		r.Post("/sections/{sectionID}/finish", h.FinishSection)
		r.Get("/question", h.CurrentQuestion)
		r.Post("/question/navigate", h.Navigate)
		r.Put("/question/response", h.SaveResponse)
		r.Post("/pause", h.Pause)
		r.Post("/resume", h.Resume)
		r.Post("/events", h.ReportEvent)
		r.Post("/submit", h.Submit)
	})
}

// RegisterStaffRoutes mounts attempt monitoring under
// /orgs/{orgID}/exams/{examID}/attempts.
func (h *Handler) RegisterStaffRoutes(r chi.Router) {
	monitor := authz.Require(authz.PermAttemptsMonitor)
	r.With(monitor).Get("/", h.ListAttempts)
	r.With(monitor).Post("/{attemptID}/release-device", h.ReleaseDevice)
}

// ---------------------------------------------------------------------------
// Session middleware
// ---------------------------------------------------------------------------

type sessionKey struct{}

func (h *Handler) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			response.Fail(w, errSessionInvalid)
			return
		}
		sess, err := h.service.Authenticate(r.Context(), strings.TrimSpace(raw), r.Header.Get(DeviceHeader))
		if err != nil {
			response.Fail(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey{}, sess)))
	})
}

func sessionFrom(r *http.Request) *Session {
	s, _ := r.Context().Value(sessionKey{}).(*Session)
	return s
}

// clientOf identifies the caller. RemoteAddr has already been rewritten by
// the trusted-proxy middleware where applicable.
func clientOf(r *http.Request) Client {
	ip := r.RemoteAddr
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	return Client{IP: ip, UserAgent: r.UserAgent()}
}

// ---------------------------------------------------------------------------
// Web portal
// ---------------------------------------------------------------------------

func (h *Handler) ListMyExams(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	list, err := h.service.ListMyExams(r.Context(), userID)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, "Exams loaded", list)
}

func (h *Handler) Launch(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	examID, err := request.URLParamUUID(r, "examID")
	if err != nil {
		response.Fail(w, err)
		return
	}
	grant, err := h.service.Launch(r.Context(), userID, examID)
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusCreated, "Launch code issued", grant)
}

// ---------------------------------------------------------------------------
// Desktop app
// ---------------------------------------------------------------------------

func (h *Handler) OpenSession(w http.ResponseWriter, r *http.Request) {
	var req OpenSessionRequest
	if errs, err := validator.DecodeAndValidate(r, &req); err != nil {
		response.Fail(w, apperr.Validation(errs))
		return
	}
	grant, err := h.service.OpenSession(r.Context(), req, clientOf(r))
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusCreated, "Session opened", grant)
}

func (h *Handler) State(w http.ResponseWriter, r *http.Request) {
	state, err := h.service.State(r.Context(), sessionFrom(r), clientOf(r))
	respond(w, err, "State loaded", state)
}

func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	state, err := h.service.Start(r.Context(), sessionFrom(r), clientOf(r))
	respond(w, err, "Exam started", state)
}

func (h *Handler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	view, err := h.service.Heartbeat(r.Context(), sessionFrom(r), clientOf(r))
	respond(w, err, "Heartbeat recorded", view)
}

func (h *Handler) EnterSection(w http.ResponseWriter, r *http.Request) {
	sectionID, err := request.URLParamUUID(r, "sectionID")
	if err != nil {
		response.Fail(w, err)
		return
	}
	view, err := h.service.EnterSection(r.Context(), sessionFrom(r), clientOf(r), sectionID)
	respond(w, err, "Section opened", view)
}

func (h *Handler) FinishSection(w http.ResponseWriter, r *http.Request) {
	sectionID, err := request.URLParamUUID(r, "sectionID")
	if err != nil {
		response.Fail(w, err)
		return
	}
	view, err := h.service.FinishSection(r.Context(), sessionFrom(r), clientOf(r), sectionID)
	respond(w, err, "Section finished", view)
}

func (h *Handler) CurrentQuestion(w http.ResponseWriter, r *http.Request) {
	view, err := h.service.CurrentQuestion(r.Context(), sessionFrom(r), clientOf(r))
	respond(w, err, "Question loaded", view)
}

func (h *Handler) Navigate(w http.ResponseWriter, r *http.Request) {
	var req NavigateRequest
	if errs, err := validator.DecodeAndValidate(r, &req); err != nil {
		response.Fail(w, apperr.Validation(errs))
		return
	}
	view, err := h.service.Navigate(r.Context(), sessionFrom(r), clientOf(r), req.Position)
	respond(w, err, "Question loaded", view)
}

func (h *Handler) SaveResponse(w http.ResponseWriter, r *http.Request) {
	var req SaveResponseRequest
	if errs, err := validator.DecodeAndValidate(r, &req); err != nil {
		response.Fail(w, apperr.Validation(errs))
		return
	}
	view, err := h.service.SaveResponse(r.Context(), sessionFrom(r), clientOf(r), req)
	respond(w, err, "Response saved", view)
}

func (h *Handler) Pause(w http.ResponseWriter, r *http.Request) {
	view, err := h.service.Pause(r.Context(), sessionFrom(r), clientOf(r))
	respond(w, err, "Exam paused", view)
}

func (h *Handler) Resume(w http.ResponseWriter, r *http.Request) {
	view, err := h.service.Resume(r.Context(), sessionFrom(r), clientOf(r))
	respond(w, err, "Exam resumed", view)
}

func (h *Handler) ReportEvent(w http.ResponseWriter, r *http.Request) {
	var req EventRequest
	if errs, err := validator.DecodeAndValidate(r, &req); err != nil {
		response.Fail(w, apperr.Validation(errs))
		return
	}
	result, err := h.service.ReportEvent(r.Context(), sessionFrom(r), clientOf(r), req)
	respond(w, err, "Event recorded", result)
}

func (h *Handler) Submit(w http.ResponseWriter, r *http.Request) {
	view, err := h.service.Submit(r.Context(), sessionFrom(r), clientOf(r))
	respond(w, err, "Exam submitted", view)
}

// ---------------------------------------------------------------------------
// Staff
// ---------------------------------------------------------------------------

func (h *Handler) ListAttempts(w http.ResponseWriter, r *http.Request) {
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
	page, err := h.service.ListAttempts(r.Context(), m, examID, p)
	respond(w, err, "Attempts loaded", page)
}

func (h *Handler) ReleaseDevice(w http.ResponseWriter, r *http.Request) {
	m, _ := authz.MembershipFrom(r.Context())
	examID, err := request.URLParamUUID(r, "examID")
	if err != nil {
		response.Fail(w, err)
		return
	}
	attemptID, err := request.URLParamUUID(r, "attemptID")
	if err != nil {
		response.Fail(w, err)
		return
	}
	err = h.service.ReleaseDevice(r.Context(), m, examID, attemptID)
	respond(w, err, "Device released; the candidate can continue on another device", nil)
}

func respond(w http.ResponseWriter, err error, message string, data any) {
	if err != nil {
		response.Fail(w, err)
		return
	}
	response.Success(w, http.StatusOK, message, data)
}
