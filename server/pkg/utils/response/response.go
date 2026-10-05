package response

import (
	"encoding/json"
	"net/http"
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
