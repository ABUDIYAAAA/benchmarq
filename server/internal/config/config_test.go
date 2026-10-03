package config_test

import (
	"os"
	"testing"

	"github.com/ABUDIYAAAA/benchmarq/internal/config"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/logger"
)

func TestConfigAndAppConfig(t *testing.T) {
	os.Setenv("PORT", "9090")
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/testdb")
	os.Setenv("ALLOWED_ORIGINS", "http://localhost:3000,http://example.com")
	os.Setenv("ENV", "test")

	cfg, err := config.NewConfig()
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.Port != "9090" {
		t.Errorf("expected port 9090, got %s", cfg.Port)
	}

	if len(cfg.AllowedOrigins) != 2 || cfg.AllowedOrigins[0] != "http://localhost:3000" || cfg.AllowedOrigins[1] != "http://example.com" {
		t.Errorf("expected allowed origins [http://localhost:3000 http://example.com], got %v", cfg.AllowedOrigins)
	}

	log := logger.NewInfoLogger()
	appCfg := config.NewAppConfig(cfg, log)

	if appCfg == nil || appCfg.Config != cfg || appCfg.Logger != log {
		t.Errorf("appConfig struct fields were not initialized correctly")
	}
}
