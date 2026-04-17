package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// ContextKeyToken is the context key for the JWT token
	ContextKeyToken contextKey = "jwt_token"
	// ContextKeyUser is the context key for the user ID
	ContextKeyUser contextKey = "user_id"
	// ContextKeyEmail is the context key for the user email
	ContextKeyEmail contextKey = "user_email"
	// ContextKeyRoles is the context key for user roles
	ContextKeyRoles contextKey = "user_roles"
)

// ─── JWKS Provider ──────────────────────────────────────────

// JWK represents a single JSON Web Key
type JWK struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// JWKS represents a JSON Web Key Set
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWKSProvider fetches and caches JWKS from Keycloak
type JWKSProvider struct {
	jwksURL    string
	keys       map[string]*rsa.PublicKey
	mu         sync.RWMutex
	lastFetch  time.Time
	cacheTTL   time.Duration
	httpClient *http.Client
	logger     *logrus.Logger
}

// NewJWKSProvider creates a JWKS provider for the given Keycloak realm
func NewJWKSProvider(keycloakURL, realm string, logger *logrus.Logger) *JWKSProvider {
	jwksURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs", keycloakURL, realm)
	logger.WithField("jwks_url", jwksURL).Info("Initializing JWKS provider")

	return &JWKSProvider{
		jwksURL:  jwksURL,
		keys:     make(map[string]*rsa.PublicKey),
		cacheTTL: 10 * time.Minute,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// GetKey returns the RSA public key for the given key ID
func (p *JWKSProvider) GetKey(kid string) (*rsa.PublicKey, error) {
	p.mu.RLock()
	if key, ok := p.keys[kid]; ok && time.Since(p.lastFetch) < p.cacheTTL {
		p.mu.RUnlock()
		return key, nil
	}
	p.mu.RUnlock()

	// Fetch fresh keys
	if err := p.refresh(); err != nil {
		return nil, fmt.Errorf("failed to refresh JWKS: %w", err)
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	key, ok := p.keys[kid]
	if !ok {
		return nil, fmt.Errorf("key not found for kid: %s", kid)
	}
	return key, nil
}

// refresh fetches the JWKS from Keycloak and parses the RSA public keys
func (p *JWKSProvider) refresh() error {
	p.logger.Debug("Fetching JWKS from Keycloak")

	resp, err := p.httpClient.Get(p.jwksURL)
	if err != nil {
		return fmt.Errorf("JWKS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("JWKS endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read JWKS response: %w", err)
	}

	var jwks JWKS
	if err := json.Unmarshal(body, &jwks); err != nil {
		return fmt.Errorf("failed to parse JWKS: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey)
	for _, jwk := range jwks.Keys {
		if jwk.Kty != "RSA" {
			continue
		}

		pubKey, err := parseRSAPublicKey(jwk)
		if err != nil {
			p.logger.WithError(err).WithField("kid", jwk.Kid).Warn("Failed to parse JWK")
			continue
		}
		keys[jwk.Kid] = pubKey
	}

	if len(keys) == 0 {
		return fmt.Errorf("no valid RSA keys found in JWKS")
	}

	p.mu.Lock()
	p.keys = keys
	p.lastFetch = time.Now()
	p.mu.Unlock()

	p.logger.WithField("key_count", len(keys)).Info("JWKS refreshed")
	return nil
}

// parseRSAPublicKey converts a JWK to an *rsa.PublicKey
func parseRSAPublicKey(jwk JWK) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}

	// Convert exponent bytes to int
	var e int
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: e,
	}, nil
}

// ─── Auth Middleware ─────────────────────────────────────────

// Middleware handles JWT token validation via JWKS
type Middleware struct {
	jwks   *JWKSProvider
	logger *logrus.Logger
}

// NewMiddleware creates a new auth middleware with JWKS validation
func NewMiddleware(jwks *JWKSProvider, logger *logrus.Logger) *Middleware {
	return &Middleware{
		jwks:   jwks,
		logger: logger,
	}
}

// ExtractToken middleware validates JWT via JWKS and extracts user claims
func (m *Middleware) ExtractToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract JWT token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			m.logger.Debug("No Authorization header found")
			next.ServeHTTP(w, r)
			return
		}

		// Parse Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			m.logger.Warn("Invalid Authorization header format")
			http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]

		// Try Istio-injected headers first (already validated at gateway)
		userID := r.Header.Get("X-Forwarded-User")
		if userID == "" {
			userID = r.Header.Get("X-Auth-Request-User")
		}

		email := r.Header.Get("X-Forwarded-Email")
		if email == "" {
			email = r.Header.Get("X-Auth-Request-Email")
		}

		rolesHeader := r.Header.Get("X-Auth-Request-Roles")
		var roles []string
		if rolesHeader != "" {
			roles = strings.Split(rolesHeader, ",")
		}

		// If no Istio headers, validate JWT via JWKS
		if userID == "" {
			claims, err := m.validateToken(tokenString)
			if err != nil {
				m.logger.WithError(err).Warn("JWT validation failed")
				http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
				return
			}

			// Extract preferred_username (Keycloak standard claim)
			if username, ok := claims["preferred_username"].(string); ok {
				userID = username
			}

			if emailClaim, ok := claims["email"].(string); ok {
				email = emailClaim
			}

			// Extract realm roles
			if realmAccess, ok := claims["realm_access"].(map[string]interface{}); ok {
				if rolesList, ok := realmAccess["roles"].([]interface{}); ok {
					for _, role := range rolesList {
						if r, ok := role.(string); ok {
							roles = append(roles, r)
						}
					}
				}
			}

			m.logger.WithFields(logrus.Fields{
				"user":  userID,
				"email": email,
			}).Debug("JWT validated via JWKS")
		} else {
			m.logger.WithFields(logrus.Fields{
				"user":  userID,
				"email": email,
				"roles": roles,
			}).Debug("Extracted user from Istio headers")
		}

		if userID == "" {
			m.logger.Warn("No user identity found in token")
			http.Error(w, "Unable to identify user from token", http.StatusUnauthorized)
			return
		}

		// Store in context for downstream handlers
		ctx := context.WithValue(r.Context(), ContextKeyToken, tokenString)
		ctx = context.WithValue(ctx, ContextKeyUser, userID)
		ctx = context.WithValue(ctx, ContextKeyEmail, email)
		ctx = context.WithValue(ctx, ContextKeyRoles, roles)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// validateToken validates a JWT using JWKS public keys from Keycloak
func (m *Middleware) validateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method is RSA
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Get key ID from token header
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("missing kid in token header")
		}

		// Look up the public key from JWKS
		return m.jwks.GetKey(kid)
	})

	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("failed to extract claims")
	}

	return claims, nil
}

// ─── Context Helpers ────────────────────────────────────────

// GetTokenFromContext extracts the JWT token from the request context
func GetTokenFromContext(ctx context.Context) string {
	if token, ok := ctx.Value(ContextKeyToken).(string); ok {
		return token
	}
	return ""
}

// GetUserFromContext extracts the user ID from the request context
func GetUserFromContext(ctx context.Context) string {
	if user, ok := ctx.Value(ContextKeyUser).(string); ok {
		return user
	}
	return ""
}

// GetEmailFromContext extracts the email from the request context
func GetEmailFromContext(ctx context.Context) string {
	if email, ok := ctx.Value(ContextKeyEmail).(string); ok {
		return email
	}
	return ""
}

// GetRolesFromContext extracts the roles from the request context
func GetRolesFromContext(ctx context.Context) []string {
	if roles, ok := ctx.Value(ContextKeyRoles).([]string); ok {
		return roles
	}
	return []string{}
}
