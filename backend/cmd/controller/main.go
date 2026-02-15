package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/devplatform/ldap-manager/internal/controller"
)

var (
	// Version information (set via ldflags)
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	// Parse flags
	kubeconfig := flag.String("kubeconfig", "", "Path to kubeconfig file")
	namespace := flag.String("namespace", "dev-platform", "Namespace for OpenLDAP")
	ldapURL := flag.String("ldap-url", "", "LDAP URL (default: internal service)")
	baseDN := flag.String("base-dn", "dc=devplatform,dc=local", "LDAP Base DN")
	adminPassword := flag.String("admin-password", "admin123", "LDAP Admin password")
	configPassword := flag.String("config-password", "config123", "LDAP Config password")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	// Show version
	if *showVersion {
		logrus.Printf("OpenLDAP Controller v%s (commit: %s, built: %s)", version, gitCommit, buildTime)
		os.Exit(0)
	}

	// Setup logger
	logger := setupLogger(*logLevel)
	logger.WithFields(logrus.Fields{
		"version":   version,
		"commit":    gitCommit,
		"buildTime": buildTime,
	}).Info("Starting OpenLDAP Controller")

	// Get Kubernetes config
	config, err := getKubeConfig(*kubeconfig)
	if err != nil {
		logger.WithError(err).Fatal("Failed to get Kubernetes config")
	}

	// Build controller config
	cfg := &controller.ControllerConfig{
		Namespace:      *namespace,
		LDAPTimeout:    30 * time.Second,
		BaseDN:         *baseDN,
		AdminDN:        "cn=admin," + *baseDN,
		AdminPassword:  *adminPassword,
		ConfigPassword: *configPassword,
		InitData:       controller.DefaultInitData(),
	}

	if *ldapURL != "" {
		cfg.LDAPURL = *ldapURL
	} else {
		cfg.LDAPURL = "ldap://openldap-internal." + *namespace + ".svc.cluster.local:389"
	}

	// Create controller
	ctrl, err := controller.NewController(config, cfg, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create controller")
	}

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start controller in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- ctrl.Run(cfg)
	}()

	// Wait for signal or error
	select {
	case sig := <-sigCh:
		logger.WithField("signal", sig.String()).Info("Received shutdown signal")
		ctrl.Stop()
	case err := <-errCh:
		if err != nil {
			logger.WithError(err).Fatal("Controller error")
		}
	}

	logger.Info("OpenLDAP Controller stopped")
}

// setupLogger configures the logger
func setupLogger(level string) *logrus.Logger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
	})
	logger.SetOutput(os.Stdout)

	switch level {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}

	return logger
}

// getKubeConfig returns the Kubernetes client config
func getKubeConfig(kubeconfig string) (*rest.Config, error) {
	// Try kubeconfig file first
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	// Try KUBECONFIG environment variable
	if env := os.Getenv("KUBECONFIG"); env != "" {
		return clientcmd.BuildConfigFromFlags("", env)
	}

	// Try in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to default kubeconfig location
		home, _ := os.UserHomeDir()
		defaultPath := home + "/.kube/config"
		if _, err := os.Stat(defaultPath); err == nil {
			return clientcmd.BuildConfigFromFlags("", defaultPath)
		}
		return nil, err
	}

	return config, nil
}
