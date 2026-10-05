package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ABUDIYAAAA/benchmarq/internal/config"
	"github.com/ABUDIYAAAA/benchmarq/internal/database"
	"github.com/ABUDIYAAAA/benchmarq/internal/mailer"
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

	// Setup context listening for termination signals
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize Database Pool
	dbPool, err := database.NewPool(ctx, cfg.DatabaseUrl)
	if err != nil {
		errorLogger.Error("Error connecting to database", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()
	infoLogger.Info("Database connection pool initialized")

	// Initialize Mailer Manager
	mailerCfg := mailer.Config{
		SMTPHost:       cfg.SMTPHost,
		SMTPPort:       cfg.SMTPPort,
		SMTPUsername:   cfg.SMTPUsername,
		SMTPPassword:   cfg.SMTPPassword,
		SMTPFromEmail:  cfg.SMTPFromEmail,
		SMTPFromName:   cfg.SMTPFromName,
		SMTPEncryption: cfg.SMTPEncryption,
		EmailWorkers:   cfg.EmailWorkers,
		EmailQueueSize: cfg.EmailQueueSize,
		Environment:    cfg.Environment,
	}
	mailManager, err := mailer.NewManager(mailerCfg, infoLogger)
	if err != nil {
		errorLogger.Error("Error initializing mailer", "error", err)
		os.Exit(1)
	}

	// Start background email worker group
	mailManager.Start(ctx)
	defer mailManager.Stop()

	// Initialize AppConfig with shared resources
	appCfg := config.NewAppConfig(cfg, infoLogger, mailManager, dbPool)

	mux := router.NewRouter(appCfg)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Port),
		Handler: mux,
	}

	go func() {
		infoLogger.Info("Server running", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errorLogger.Error("Server error", "error", err)
		}
	}()

	<-ctx.Done()
	infoLogger.Info("Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		errorLogger.Error("Server forced shutdown", "error", err)
	}

	infoLogger.Info("Server exited cleanly")
}
