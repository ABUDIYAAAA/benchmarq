package middleware

import "net/http"

// SecurityHeaders returns a middleware that attaches standard HTTP security headers to all incoming responses.
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Prevent MIME type sniffing
			w.Header().Set("X-Content-Type-Options", "nosniff")
			// Prevent clickjacking by denying framing
			w.Header().Set("X-Frame-Options", "DENY")
			// Cross-site scripting (XSS) filter protection for legacy browsers
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			// Control referrer information sent in HTTP requests
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			// Restrict browser features and APIs
			w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			// Basic Content Security Policy
			w.Header().Set("Content-Security-Policy", "default-src 'self'")

			// If HTTPS, enforce Strict-Transport-Security (HSTS)
			if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
			}

			next.ServeHTTP(w, r)
		})
	}
}
