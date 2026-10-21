package router

import (
	"net/http"

	"github.com/ABUDIYAAAA/benchmarq/internal/authz"
	"github.com/ABUDIYAAAA/benchmarq/internal/config"
	"github.com/ABUDIYAAAA/benchmarq/internal/database"
	"github.com/ABUDIYAAAA/benchmarq/internal/middleware"
	"github.com/ABUDIYAAAA/benchmarq/internal/modules/assignments"
	"github.com/ABUDIYAAAA/benchmarq/internal/modules/attempts"
	"github.com/ABUDIYAAAA/benchmarq/internal/modules/auth"
	"github.com/ABUDIYAAAA/benchmarq/internal/modules/exams"
	"github.com/ABUDIYAAAA/benchmarq/internal/modules/organizations"
	"github.com/ABUDIYAAAA/benchmarq/internal/modules/questions"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/request"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/response"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
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

	tx := database.NewTransactor(appCfg.DB)

	// API v1 Routes
	r.Route("/api/v1", func(api chi.Router) {
		api.Use(chimiddleware.RequestSize(request.MaxBodyBytes))

		// Initialize Auth module
		authRepo := auth.NewRepository(appCfg.DB)
		authService := auth.NewService(authRepo, appCfg.Config, appCfg.Logger, appCfg.Mailer)
		authHandler := auth.NewHandler(authService, appCfg.Config)

		authHandler.RegisterRoutes(api, authMiddleware)

		// Organizations, invitations, exams, questions and assignments
		orgRepo := organizations.NewRepository(appCfg.DB)
		orgHandler := organizations.NewHandler(organizations.NewService(orgRepo, tx, appCfg.Logger, organizations.InviteOptions{
			Mailer:      appCfg.Mailer,
			FrontendURL: appCfg.Config.FrontendURL,
			ExpiryDays:  appCfg.Config.OrgInviteExpiryDays,
		}))
		examHandler := exams.NewHandler(exams.NewService(exams.NewRepository(appCfg.DB), tx, appCfg.Logger))
		questionHandler := questions.NewHandler(questions.NewService(questions.NewRepository(appCfg.DB), tx, appCfg.Logger))
		assignmentHandler := assignments.NewHandler(assignments.NewService(assignments.NewRepository(appCfg.DB), tx, appCfg.Logger, assignments.Options{
			Mailer:      appCfg.Mailer,
			FrontendURL: appCfg.Config.FrontendURL,
		}))

		attemptHandler := attempts.NewHandler(NewAttemptService(appCfg))

		api.Route("/invitations", func(inv chi.Router) {
			inv.Use(authMiddleware)
			orgHandler.RegisterInvitationRoutes(inv)
		})

		// Candidate portal (browser cookies) and the exam desktop app
		// (session tokens bound to a device).
		api.Route("/me/exams", func(me chi.Router) {
			me.Use(authMiddleware)
			attemptHandler.RegisterPortalRoutes(me)
		})
		api.Route("/runtime", attemptHandler.RegisterRuntimeRoutes)

		api.Route("/orgs", func(orgs chi.Router) {
			orgs.Use(authMiddleware)
			orgHandler.RegisterRoutes(orgs)

			orgs.Route("/{orgID}", func(org chi.Router) {
				org.Use(authz.RequireMembership(orgRepo))
				orgHandler.RegisterOrgRoutes(org)

				org.Route("/exams", func(r chi.Router) {
					examHandler.RegisterRoutes(r, exams.Subroutes{
						Questions:   questionHandler.RegisterRoutes,
						Assignments: assignmentHandler.RegisterRoutes,
						Attempts:    attemptHandler.RegisterStaffRoutes,
					})
				})
			})
		})
	})

	return r
}

// NewAttemptService builds the attempt runtime. The server uses it both for
// routes and for the background sweeper.
func NewAttemptService(appCfg *config.AppConfig) attempts.Service {
	return attempts.NewService(
		attempts.NewRepository(appCfg.DB),
		exams.NewRepository(appCfg.DB),
		questions.NewRepository(appCfg.DB),
		database.NewTransactor(appCfg.DB),
		appCfg.Logger,
		attempts.Options{DesktopLaunchURL: appCfg.Config.DesktopLaunchURL},
	)
}
