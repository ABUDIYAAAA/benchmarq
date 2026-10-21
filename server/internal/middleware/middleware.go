package middleware

import (
	"github.com/ABUDIYAAAA/benchmarq/internal/config"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// RegisterGlobalMiddlewares applies all core middlewares to the provided chi router.
// Middlewares applied:
// 1. RequestID - generates unique ID per request
// 2. TrustedRealIP - client IP from X-Forwarded-For / X-Real-IP, only when sent by a trusted proxy
// 3. CleanPath - cleans duplicate slashes in request paths
// 4. SecurityHeaders - adds security headers (XSS, HSTS, frame options, etc.)
// 5. CORS - enables configurable cross-origin resource sharing from .env / AppConfig
// 6. RequestLogger - structured slog request logging
// 7. Recoverer - catches panics and logs stack trace with slog
// 8. Compress - transparent response gzip compression
func RegisterGlobalMiddlewares(r *chi.Mux, appCfg *config.AppConfig) {
	// 1. Chi standard request tracking & path cleaning
	r.Use(chimiddleware.RequestID)
	if appCfg != nil && appCfg.Config != nil {
		r.Use(TrustedRealIP(ParsePrefixes(appCfg.Config.TrustedProxies)))
	}
	r.Use(chimiddleware.CleanPath)

	// 2. Security headers
	r.Use(SecurityHeaders())

	// 3. CORS configured from AppConfig
	if appCfg != nil && appCfg.Config != nil {
		r.Use(CORS(appCfg.Config.AllowedOrigins))
	} else {
		r.Use(CORS([]string{"*"}))
	}

	// 4. Structured logging and panic recovery
	if appCfg != nil && appCfg.Logger != nil {
		r.Use(RequestLogger(appCfg.Logger))
		r.Use(Recoverer(appCfg.Logger))
	}

	// 5. Response compression
	r.Use(Compress(5))
}
