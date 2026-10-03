package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// Recoverer recovers from panics, logs the panic details and stack trace with slog, and sends a 500 JSON response.
func Recoverer(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rvr := recover(); rvr != nil {
					if rvr == http.ErrAbortHandler {
						// Don't recover http.ErrAbortHandler (standard Go behavior)
						panic(rvr)
					}

					reqID := chimiddleware.GetReqID(r.Context())
					stack := string(debug.Stack())

					if log != nil {
						log.Error("HTTP request panic recovered",
							slog.String("request_id", reqID),
							slog.Any("panic", rvr),
							slog.String("method", r.Method),
							slog.String("path", r.URL.Path),
							slog.String("stack", stack),
						)
					}

					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]string{
						"error":      "Internal Server Error",
						"request_id": reqID,
					})
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
