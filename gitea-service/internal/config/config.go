package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config holds all configuration for the Gitea service
type Config struct {
	// Gitea configuration
	GiteaURL   string `envconfig:"GITEA_URL" required:"true"`
	GiteaToken string `envconfig:"GITEA_TOKEN"` // Optional - auto-generated if empty

	// Gitea admin credentials (for auto token generation)
	GiteaAdminUser     string `envconfig:"GITEA_ADMIN_USER" default:"gitea_admin"`
	GiteaAdminPassword string `envconfig:"GITEA_ADMIN_PASSWORD" default:"Admin123!"`
	GiteaAdminEmail    string `envconfig:"GITEA_ADMIN_EMAIL" default:"admin@local.dev"`

	// Gitea default owner/organization for all repositories
	// This is the admin user in Gitea that owns all repos
	// Access control is handled via LDAP, not Gitea's user system
	GiteaDefaultOwner string `envconfig:"GITEA_DEFAULT_OWNER" default:"gitea_admin"`

	// LDAP Manager service configuration (for inter-service communication)
	LDAPManagerURL string `envconfig:"LDAP_MANAGER_URL" required:"true"`

	// Server configuration
	Port        int    `envconfig:"PORT" default:"8081"`
	MetricsPort int    `envconfig:"METRICS_PORT" default:"9091"`
	Environment string `envconfig:"ENVIRONMENT" default:"development"`
	LogLevel    string `envconfig:"LOG_LEVEL" default:"info"`

	// JWT configuration (must match LDAP Manager for token validation)
	JWTSecret     string        `envconfig:"JWT_SECRET" required:"true"`
	JWTExpiration time.Duration `envconfig:"JWT_EXPIRATION" default:"24h"`

	// Gitea webhook secret (for validating webhook signatures)
	GiteaWebhookSecret string `envconfig:"GITEA_WEBHOOK_SECRET" default:""`

	// CORS configuration
	CORSOrigins []string `envconfig:"CORS_ORIGINS" default:"*"`

	// Graceful shutdown timeout
	ShutdownTimeout int `envconfig:"SHUTDOWN_TIMEOUT" default:"30"`

	// HTTP client timeouts
	HTTPClientTimeout time.Duration `envconfig:"HTTP_CLIENT_TIMEOUT" default:"30s"`

	// Reconciliation controller configuration
	ReconcileInterval    time.Duration `envconfig:"RECONCILE_INTERVAL" default:"5m"`
	WebhookCheckInterval time.Duration `envconfig:"WEBHOOK_CHECK_INTERVAL" default:"2m"`
	WebhookTargetHost    string        `envconfig:"WEBHOOK_TARGET_HOST" default:"gitea-service.dev-platform.svc.cluster.local:8081"`
	ReconcileEnabled     bool          `envconfig:"RECONCILE_ENABLED" default:"true"`

	// Group sync interval (LDAP groups/departments → Gitea teams)
	GroupSyncInterval time.Duration `envconfig:"GROUP_SYNC_INTERVAL" default:"5m"`

	// Persistent state directory (for controller StatefulSet)
	DataDir string `envconfig:"DATA_DIR" default:"/data"`

	// User sync configuration (LDAP → Gitea automatic sync)
	UserSyncDefaultPassword string `envconfig:"USER_SYNC_DEFAULT_PASSWORD" default:"changeme123!"`

	// Keycloak configuration (for controller service account tokens)
	KeycloakURL          string `envconfig:"KEYCLOAK_URL" default:"http://keycloak.auth-system.svc.cluster.local:8080"`
	KeycloakRealm        string `envconfig:"KEYCLOAK_REALM" default:"devplatform"`
	KeycloakClientID     string `envconfig:"KEYCLOAK_CLIENT_ID" default:"gitea-service"`
	KeycloakClientSecret string `envconfig:"KEYCLOAK_CLIENT_SECRET"`
}

// GetKeycloakTokenURL returns the Keycloak token endpoint
func (c *Config) GetKeycloakTokenURL() string {
	return fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", c.KeycloakURL, c.KeycloakRealm)
}

// Load reads configuration from environment variables
func Load() *Config {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		panic(fmt.Sprintf("failed to load configuration: %v", err))
	}

	return &cfg
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// GetDefaultOwner returns the default Gitea owner (admin user)
// This centralizes the logic for determining repo ownership
// All repositories are owned by this user, access control is via LDAP
func (c *Config) GetDefaultOwner() string {
	if c.GiteaDefaultOwner == "" {
		return "admin123" // Fallback default
	}
	return c.GiteaDefaultOwner
}
