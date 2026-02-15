package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/devplatform/gitea-service/internal/config"
	"github.com/devplatform/gitea-service/internal/gitea"
	"github.com/devplatform/gitea-service/internal/ldap"
	gosync "github.com/devplatform/gitea-service/internal/sync"
	"github.com/sirupsen/logrus"
)

func main() {
	// Load configuration (same env vars as the server)
	cfg := config.Load()

	// Setup logger
	logger := setupLogger(cfg)
	logger.Info("Starting Gitea Sync Controller")

	// Get or generate Gitea token
	giteaToken := cfg.GiteaToken
	if giteaToken == "" {
		logger.Info("GITEA_TOKEN not set, auto-generating...")
		initClient := gitea.NewInitClient(&gitea.InitConfig{
			GiteaURL:      cfg.GiteaURL,
			AdminUser:     cfg.GiteaAdminUser,
			AdminPassword: cfg.GiteaAdminPassword,
			AdminEmail:    cfg.GiteaAdminEmail,
			TokenName:     "gitea-controller-token",
		}, logger)

		var err error
		giteaToken, err = initClient.EnsureAdminToken()
		if err != nil {
			logger.WithError(err).Fatal("Failed to initialize Gitea token")
		}
	}

	// Initialize Gitea client
	logger.Info("Initializing Gitea client")
	giteaClient := gitea.NewClient(cfg.GiteaURL, giteaToken, logger)

	// Test Gitea connection
	if err := giteaClient.HealthCheck(); err != nil {
		logger.WithError(err).Warn("Initial Gitea health check failed")
	} else {
		logger.Info("Gitea connection successful")
	}

	// Initialize LDAP Manager client
	logger.Info("Initializing LDAP Manager client")
	ldapClient := ldap.NewClient(cfg.LDAPManagerURL, cfg.HTTPClientTimeout, logger)

	// Test LDAP Manager connection
	ctx := context.Background()
	if err := ldapClient.HealthCheck(ctx); err != nil {
		logger.WithError(err).Warn("Initial LDAP Manager health check failed")
	} else {
		logger.Info("LDAP Manager connection successful")
	}

	// Initialize Gitea service (for sync operations)
	logger.Info("Initializing Gitea service")
	giteaService := gitea.NewService(giteaClient, ldapClient, logger)

	// Create the reconciliation controller
	controller := gosync.NewController(giteaService, giteaClient, ldapClient, cfg, logger)

	// Setup HTTP server for webhook + health + metrics
	mux := http.NewServeMux()
	controller.SetupHTTPHandlers(mux)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start the controller goroutines
	go controller.Start()

	// Start HTTP server
	go func() {
		logger.WithField("port", cfg.Port).Info("Starting controller HTTP server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("Controller HTTP server failed to start")
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	sig := <-quit
	logger.WithField("signal", sig.String()).Info("Shutdown signal received")

	// Stop controller goroutines
	controller.Stop()

	// Shutdown HTTP server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger.Info("Shutting down HTTP server...")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.WithError(err).Error("Server shutdown failed")
	}

	logger.Info("Gitea Sync Controller stopped")
}

func setupLogger(cfg *config.Config) *logrus.Logger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "level",
			logrus.FieldKeyMsg:   "message",
		},
	})
	logger.SetOutput(os.Stdout)

	if cfg.LogLevel == "debug" {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	return logger
}
