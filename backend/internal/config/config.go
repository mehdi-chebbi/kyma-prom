package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config holds all configuration for the LDAP manager service
type Config struct {
	// LDAP configuration
	LDAPURL             string        `envconfig:"LDAP_URL" required:"true"`
	LDAPBaseDN          string        `envconfig:"LDAP_BASE_DN" required:"true"`
	LDAPBindDN          string        `envconfig:"LDAP_BIND_DN" required:"true"`
	LDAPBindPassword    string        `envconfig:"LDAP_BIND_PASSWORD" required:"true"`
	LDAPPoolSize        int           `envconfig:"LDAP_POOL_SIZE" default:"10"`
	LDAPPoolTimeout     time.Duration `envconfig:"LDAP_POOL_TIMEOUT" default:"30s"`
	LDAPConnTimeout     time.Duration `envconfig:"LDAP_CONN_TIMEOUT" default:"10s"`
	LDAPMaxConnLifetime time.Duration `envconfig:"LDAP_MAX_CONN_LIFETIME" default:"30m"`

	// Server configuration
	Port        int    `envconfig:"PORT" default:"8080"`
	MetricsPort int    `envconfig:"METRICS_PORT" default:"9090"`
	Environment string `envconfig:"ENVIRONMENT" default:"development"`
	LogLevel    string `envconfig:"LOG_LEVEL" default:"info"`

	// JWT configuration
	JWTSecret     string        `envconfig:"JWT_SECRET" required:"true"`
	JWTExpiration time.Duration `envconfig:"JWT_EXPIRATION" default:"24h"`
	// to redo as mtls or both

	// CORS configuration
	CORSOrigins []string `envconfig:"CORS_ORIGINS" default:"*"`

	// Graceful shutdown timeout
	ShutdownTimeout int `envconfig:"SHUTDOWN_TIMEOUT" default:"30"`

	// Starting UID and GID for auto-increment
	StartingUID int `envconfig:"STARTING_UID" default:"10000"`
	StartingGID int `envconfig:"STARTING_GID" default:"10000"`
}

// Load reads configuration from environment variables
func Load() *Config {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		panic(fmt.Sprintf("failed to load configuration: %v", err))
	}

	return &cfg
}

// UserDN returns the full DN for a user
func (c *Config) UserDN(uid string) string {
	return fmt.Sprintf("uid=%s,ou=users,%s", uid, c.LDAPBaseDN)
}

// DepartmentDN returns the full DN for a department
func (c *Config) DepartmentDN(ou string) string {
	return fmt.Sprintf("ou=%s,ou=departments,%s", ou, c.LDAPBaseDN)
}

// GroupDN returns the full DN for a group
func (c *Config) GroupDN(cn string) string {
	return fmt.Sprintf("cn=%s,ou=groups,%s", cn, c.LDAPBaseDN)
}

// UsersDN returns the base DN for all users
func (c *Config) UsersDN() string {
	return fmt.Sprintf("ou=users,%s", c.LDAPBaseDN)
}

// DepartmentsDN returns the base DN for all departments
func (c *Config) DepartmentsDN() string {
	return fmt.Sprintf("ou=departments,%s", c.LDAPBaseDN)
}

// GroupsDN returns the base DN for all groups
func (c *Config) GroupsDN() string {
	return fmt.Sprintf("ou=groups,%s", c.LDAPBaseDN)
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}
