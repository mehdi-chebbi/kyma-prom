package config

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

// Config holds all configuration for the service
type Config struct {
	// Server settings
	Port            int    `envconfig:"PORT" default:"8082"`
	MetricsPort     int    `envconfig:"METRICS_PORT" default:"9092"`
	Environment     string `envconfig:"ENVIRONMENT" default:"development"`
	LogLevel        string `envconfig:"LOG_LEVEL" default:"info"`
	ShutdownTimeout int    `envconfig:"SHUTDOWN_TIMEOUT" default:"30"`

	// Kubernetes settings
	KubeConfig      string `envconfig:"KUBECONFIG" default:""`
	Namespace       string `envconfig:"CODESERVER_NAMESPACE" default:"codeserver-instances"`
	PVCStorageClass string `envconfig:"PVC_STORAGE_CLASS" default:"standard"`
	PVCSize         string `envconfig:"PVC_SIZE" default:"10Gi"`

	// Code-Server settings
	CodeServerImage   string `envconfig:"CODESERVER_IMAGE" default:"codercom/code-server:latest"`
	CodeServerCPU     string `envconfig:"CODESERVER_CPU_REQUEST" default:"500m"`
	CodeServerMemory  string `envconfig:"CODESERVER_MEMORY_REQUEST" default:"1Gi"`
	CodeServerCPUMax  string `envconfig:"CODESERVER_CPU_LIMIT" default:"2"`
	CodeServerMemMax  string `envconfig:"CODESERVER_MEMORY_LIMIT" default:"4Gi"`
	CodeServerTimeout int    `envconfig:"CODESERVER_TIMEOUT" default:"300"`

	// Gitea settings
	GiteaURL   string `envconfig:"GITEA_URL" required:"true"`
	GiteaToken string `envconfig:"GITEA_TOKEN" required:"true"`

	// Gitea Service (for access validation via GraphQL)
	GiteaServiceURL string `envconfig:"GITEA_SERVICE_URL" required:"true"`

	// Keycloak settings (for Istio JWT validation config)
	KeycloakURL   string `envconfig:"KEYCLOAK_URL" default:"https://keycloak.devplatform.local"`
	KeycloakRealm string `envconfig:"KEYCLOAK_REALM" default:"devplatform"`

	// Domain settings
	BaseDomain string `envconfig:"BASE_DOMAIN" default:"devplatform.local"`
	UseHTTPS   bool   `envconfig:"USE_HTTPS" default:"true"`

	// CORS settings
	CORSOrigins string `envconfig:"CORS_ORIGINS" default:"*"`
}

// GetKeycloakIssuer returns the Keycloak issuer URL
func (c *Config) GetKeycloakIssuer() string {
	return fmt.Sprintf("%s/realms/%s", c.KeycloakURL, c.KeycloakRealm)
}

// GetKeycloakJWKSURL returns the Keycloak JWKS endpoint for Istio config
func (c *Config) GetKeycloakJWKSURL() string {
	return fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs", c.KeycloakURL, c.KeycloakRealm)
}

// Load loads configuration from environment variables
func Load() *Config {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		panic(fmt.Sprintf("failed to load configuration: %v", err))
	}
	return &cfg
}

// GetCodeServerURL returns the URL for a user's code-server instance
func (c *Config) GetCodeServerURL(userID string) string {
	protocol := "http"
	if c.UseHTTPS {
		protocol = "https"
	}
	return fmt.Sprintf("%s://code-%s.%s", protocol, sanitizeUserID(userID), c.BaseDomain)
}

// GetPodName returns the pod name for a user
func (c *Config) GetPodName(userID string) string {
	return fmt.Sprintf("code-server-%s", sanitizeUserID(userID))
}

// GetPVCName returns the PVC name for a user
func (c *Config) GetPVCName(userID string) string {
	return fmt.Sprintf("workspace-%s", sanitizeUserID(userID))
}

// GetServiceName returns the service name for a user
func (c *Config) GetServiceName(userID string) string {
	return fmt.Sprintf("code-server-%s", sanitizeUserID(userID))
}

// sanitizeUserID converts a user ID to a valid Kubernetes resource name
func sanitizeUserID(userID string) string {
	// Replace dots and underscores with dashes, lowercase
	result := make([]byte, 0, len(userID))
	for i := 0; i < len(userID); i++ {
		c := userID[i]
		switch {
		case c >= 'a' && c <= 'z':
			result = append(result, c)
		case c >= 'A' && c <= 'Z':
			result = append(result, c+32) // lowercase
		case c >= '0' && c <= '9':
			result = append(result, c)
		case c == '.' || c == '_' || c == '@':
			result = append(result, '-')
		case c == '-':
			result = append(result, c)
		}
	}
	// Ensure it doesn't start or end with a dash
	if len(result) > 0 && result[0] == '-' {
		result = result[1:]
	}
	if len(result) > 0 && result[len(result)-1] == '-' {
		result = result[:len(result)-1]
	}
	// Truncate to 63 characters (Kubernetes limit)
	if len(result) > 63 {
		result = result[:63]
	}
	return string(result)
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}
