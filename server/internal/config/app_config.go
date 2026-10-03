package config

import (
	"log/slog"
)

// AppConfig holds core application-wide shared resources, configuration, and dependencies.
// This makes passing down instances such as loggers, mailers, and databases clean and maintainable.
type AppConfig struct {
	Config *Config
	Logger *slog.Logger
	// Mailer can be added here once mailer service is implemented
}

// NewAppConfig creates and returns a new AppConfig instance.
func NewAppConfig(cfg *Config, log *slog.Logger) *AppConfig {
	return &AppConfig{
		Config: cfg,
		Logger: log,
	}
}
