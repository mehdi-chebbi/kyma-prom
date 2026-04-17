package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	ContextKeyToken contextKey = "jwt_token"
	ContextKeyUser  contextKey = "user_id"
	ContextKeyEmail contextKey = "user_email"
	ContextKeyRoles contextKey = "user_roles"
)

// Middleware handles JWT token extraction from Istio headers
type Middleware struct {
	logger *logrus.Logger
}

// NewMiddleware creates a new auth middleware
func NewMiddleware(logger *logrus.Logger) *Middleware {
	return &Middleware{
		logger: logger,
	}
}

// ExtractToken middleware extracts JWT token and user claims from Istio headers
// Istio RequestAuthentication validates the JWT and adds these headers:
// - Authorization: Bearer <token>
// - X-JWT-Payload: <base64-encoded claims>
// - X-Forwarded-User: <username>
// - X-Forwarded-Email: <email>
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

		token := parts[1]

		// Try to extract user from Istio-injected headers first
		userID := r.Header.Get("X-Forwarded-User")
		if userID == "" {
			userID = r.Header.Get("X-Auth-Request-User")
		}

		email := r.Header.Get("X-Forwarded-Email")
		if email == "" {
			email = r.Header.Get("X-Auth-Request-Email")
		}

		// Extract roles from header (comma-separated)
		rolesHeader := r.Header.Get("X-Auth-Request-Roles")
		var roles []string
		if rolesHeader != "" {
			roles = strings.Split(rolesHeader, ",")
		}

		// If Istio headers not present, decode JWT claims (Postman mode)
		if userID == "" {
			claims, err := m.decodeJWT(token)
			if err != nil {
				m.logger.WithError(err).Debug("Failed to decode JWT - allowing request")
				next.ServeHTTP(w, r)
				return
			}

			if username, ok := claims["preferred_username"].(string); ok {
				userID = username
			}

			if emailClaim, ok := claims["email"].(string); ok {
				email = emailClaim
			}

			m.logger.WithFields(logrus.Fields{
				"user":  userID,
				"email": email,
			}).Debug("Extracted user from JWT claims (Postman mode)")
		} else {
			m.logger.WithFields(logrus.Fields{
				"user":  userID,
				"email": email,
				"roles": roles,
			}).Debug("Extracted user from Istio headers")
		}

		// Store in context for downstream handlers
		ctx := context.WithValue(r.Context(), ContextKeyToken, token)
		ctx = context.WithValue(ctx, ContextKeyUser, userID)
		ctx = context.WithValue(ctx, ContextKeyEmail, email)
		ctx = context.WithValue(ctx, ContextKeyRoles, roles)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

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

// decodeJWT decodes a JWT token without validation (for Postman testing)
func (m *Middleware) decodeJWT(token string) (map[string]interface{}, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}

	return claims, nil
}
