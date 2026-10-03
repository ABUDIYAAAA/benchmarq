package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/ABUDIYAAAA/benchmarq/internal/config"
	"github.com/ABUDIYAAAA/benchmarq/internal/router"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/logger"
)

func main() {
	infoLogger := logger.NewInfoLogger()
	errorLogger := logger.NewErrorLogger()

	cfg, err := config.NewConfig()
	if err != nil {
		errorLogger.Error("Error loading config", "error", err)
		os.Exit(1)
	}

	appCfg := config.NewAppConfig(cfg, infoLogger)

	mux := router.NewRouter(appCfg)

	infoLogger.Info("Server running", "port", cfg.Port)
	err = http.ListenAndServe(fmt.Sprintf(":%s", cfg.Port), mux)
	if err != nil {
		errorLogger.Error("Error running server", "error", err)
		os.Exit(1)
	}
}
