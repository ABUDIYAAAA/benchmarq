package middleware

import (
	"net/http"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// Compress returns Chi's compression middleware with the specified gzip compression level.
// Recommended level is 5 (balanced performance and size).
func Compress(level int, types ...string) func(http.Handler) http.Handler {
	if level <= 0 || level > 9 {
		level = 5
	}
	return chimiddleware.Compress(level, types...)
}
