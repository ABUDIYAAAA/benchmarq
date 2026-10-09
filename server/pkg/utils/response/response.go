package response

import (
	"encoding/json"
	"net/http"

	"github.com/ABUDIYAAAA/benchmarq/pkg/apperr"
)

type Response struct {
	Success bool              `json:"success"`
	Message string            `json:"message,omitempty"`
	Data    any               `json:"data,omitempty"`
	Errors  map[string]string `json:"errors,omitempty"`
}

// JSON sends a JSON response with status code and payload.
func JSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// Success sends a standard success JSON response.
func Success(w http.ResponseWriter, status int, message string, data any) {
	JSON(w, status, Response{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// Error sends a standard error JSON response.
func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, Response{
		Success: false,
		Message: message,
	})
}

// ValidationError sends a 422 Unprocessable Entity response with field-level validation errors.
func ValidationError(w http.ResponseWriter, errors map[string]string) {
	JSON(w, http.StatusUnprocessableEntity, Response{
		Success: false,
		Message: "Validation failed",
		Errors:  errors,
	})
}

// Fail maps an application error to the matching HTTP status and payload.
// Internal errors are reported with a generic message so causes never leak to clients.
func Fail(w http.ResponseWriter, err error) {
	e, ok := apperr.As(err)
	if !ok {
		Error(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	switch e.Kind {
	case apperr.KindInvalid:
		JSON(w, http.StatusUnprocessableEntity, Response{
			Success: false,
			Message: e.Message,
			Errors:  e.Fields,
		})
	case apperr.KindNotFound:
		Error(w, http.StatusNotFound, e.Message)
	case apperr.KindConflict:
		Error(w, http.StatusConflict, e.Message)
	case apperr.KindForbidden:
		Error(w, http.StatusForbidden, e.Message)
	case apperr.KindUnauthorized:
		Error(w, http.StatusUnauthorized, e.Message)
	default:
		Error(w, http.StatusInternalServerError, "Internal server error")
	}
}
