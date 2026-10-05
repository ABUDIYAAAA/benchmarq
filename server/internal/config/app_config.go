package config

import (
	"log/slog"

	"github.com/ABUDIYAAAA/benchmarq/internal/mailer"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AppConfig holds core application-wide shared resources, configuration, and dependencies.
// This makes passing down instances such as loggers, mailers, and databases clean and maintainable.
type AppConfig struct {
	Config *Config
	Logger *slog.Logger
	Mailer mailer.Mailer
	DB     *pgxpool.Pool
}

// NewAppConfig creates and returns a new AppConfig instance.
func NewAppConfig(cfg *Config, log *slog.Logger, m mailer.Mailer, db *pgxpool.Pool) *AppConfig {
	return &AppConfig{
		Config: cfg,
		Logger: log,
		Mailer: m,
		DB:     db,
	}
}
