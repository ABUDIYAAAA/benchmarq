package router

import (
	"github.com/ABUDIYAAAA/benchmarq/internal/config"
	"github.com/ABUDIYAAAA/benchmarq/internal/middleware"
	"github.com/go-chi/chi/v5"
)

func NewRouter(appCfg *config.AppConfig) *chi.Mux {
	r := chi.NewMux()

	// Register global middlewares
	middleware.RegisterGlobalMiddlewares(r, appCfg)

	return r
}
