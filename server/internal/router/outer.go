package router

import (
	"net/http"

	"github.com/ABUDIYAAAA/benchmarq/internal/config"
	"github.com/ABUDIYAAAA/benchmarq/internal/middleware"
	"github.com/ABUDIYAAAA/benchmarq/internal/modules/auth"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/response"
	"github.com/go-chi/chi/v5"
)

func NewRouter(appCfg *config.AppConfig) *chi.Mux {
	r := chi.NewMux()

	// Register global middlewares (CORS, Logger, Recoverer, Security, Compress)
	middleware.RegisterGlobalMiddlewares(r, appCfg)

	// Auth Middleware
	authMiddleware := middleware.AuthRequired(appCfg)

	// Health Check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		response.Success(w, http.StatusOK, "Benchmarq API is healthy", map[string]string{
			"environment": appCfg.Config.Environment,
			"version":     "v1",
		})
	})

	// API v1 Routes
	r.Route("/api/v1", func(api chi.Router) {
		// Initialize Auth module
		authRepo := auth.NewRepository(appCfg.DB)
		authService := auth.NewService(authRepo, appCfg.Config, appCfg.Logger, appCfg.Mailer)
		authHandler := auth.NewHandler(authService, appCfg.Config)

		authHandler.RegisterRoutes(api, authMiddleware)
	})

	return r
}
