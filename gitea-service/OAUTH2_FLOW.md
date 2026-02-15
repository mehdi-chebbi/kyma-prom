# OAuth2 Token Passthrough Architecture

## Overview

The gitea-service uses a **clean OAuth2 token passthrough** architecture where:
- **One JWT token** issued by Keycloak is used for everything
- **Istio validates** the JWT at the ingress gateway
- **Services trust** Istio-validated tokens
- **No dual-token complexity** - single authentication flow

## Architecture Diagram

```
┌──────────────────────────────────────────────────────────┐
│                     USER/CLIENT                          │
└────────────────────────┬─────────────────────────────────┘
                         │
                         │ 1. Login to get JWT
                         ▼
           ┌──────────────────────────────┐
           │         Keycloak             │
           │   (Authorization Server)     │
           │                              │
           │  POST /realms/devplatform/   │
           │       protocol/openid-       │
           │       connect/token          │
           └──────────────┬───────────────┘
                          │
                          │ 2. Returns JWT Token
                          ▼
┌──────────────────────────────────────────────────────────┐
│                    CLIENT STORES TOKEN                   │
│          Authorization: Bearer <jwt-token>               │
└────────────────────────┬─────────────────────────────────┘
                         │
                         │ 3. GraphQL Request with JWT
                         ▼
           ┌──────────────────────────────┐
           │   Istio Ingress Gateway      │
           │  ========================     │
           │  RequestAuthentication:      │
           │  - Validate JWT signature    │
           │  - Check issuer (Keycloak)   │
           │  - Verify expiration         │
           │  - Extract claims            │
           │  - Add headers:              │
           │    X-Forwarded-User          │
           │    X-Forwarded-Email         │
           │    X-JWT-Payload             │
           └──────────────┬───────────────┘
                          │
                          │ 4. Validated request
                          │    with enriched headers
                          ▼
           ┌──────────────────────────────┐
           │      gitea-service           │
           │   (GraphQL API)              │
           │  ========================     │
           │  Middleware extracts:        │
           │  - Token from Authorization  │
           │  - User from X-Forwarded-*   │
           │  - Stores in context         │
           └──────────────┬───────────────┘
                          │
                          │ 5. Pass same token to Gitea
                          │    Authorization: Bearer <jwt>
                          ▼
           ┌──────────────────────────────┐
           │         Gitea API            │
           │   (Resource Server)          │
           │  ========================     │
           │  OAuth2 Client validates:    │
           │  - JWT signature (JWKS)      │
           │  - User permissions          │
           │  - Returns protected data    │
           └──────────────────────────────┘
```

## Token Flow Explained

### Step 1: User Login

**Request:**
```bash
POST http://keycloak.localhost/realms/devplatform/protocol/openid-connect/token
Content-Type: application/x-www-form-urlencoded

grant_type=password
client_id=gitea-service
client_secret=<client-secret>
username=john.doe
password=password123
```

**Response:**
```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in": 900,
  "refresh_expires_in": 1800,
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI...",
  "token_type": "Bearer",
  "not-before-policy": 0,
  "session_state": "abc123...",
  "scope": "openid profile email"
}
```

### Step 2: GraphQL Request

**Client stores token and uses it:**
```bash
POST http://gitea-service.localhost/graphql
Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...
Content-Type: application/json

{
  "query": "{ listRepositories { items { name owner { username } } } }"
}
```

### Step 3: Istio Validates JWT

**Istio RequestAuthentication config:**
```yaml
apiVersion: security.istio.io/v1beta1
kind: RequestAuthentication
metadata:
  name: keycloak-jwt
  namespace: istio-system
spec:
  jwtRules:
  - issuer: "http://keycloak.auth-system:8080/realms/devplatform"
    jwksUri: "http://keycloak.auth-system:8080/realms/devplatform/protocol/openid-connect/certs"
    forwardOriginalToken: true
    outputPayloadToHeader: "x-jwt-payload"
```

**What Istio does:**
1. Fetches public keys from Keycloak JWKS endpoint
2. Validates JWT signature using RS256 algorithm
3. Checks token not expired (`exp` claim)
4. Verifies issuer matches Keycloak
5. Extracts claims and adds headers:
   - `X-Forwarded-User: john.doe`
   - `X-Forwarded-Email: john.doe@devplatform.local`
   - `X-JWT-Payload: <base64-encoded claims>`
6. Forwards request to gitea-service

**If validation fails:**
- Istio returns `401 Unauthorized`
- Request never reaches gitea-service

### Step 4: gitea-service Middleware

**Middleware extracts user context:**
```go
func (m *Middleware) ExtractToken(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Extract token
        authHeader := r.Header.Get("Authorization")
        token := strings.TrimPrefix(authHeader, "Bearer ")

        // Extract user info from Istio headers
        userID := r.Header.Get("X-Forwarded-User")
        email := r.Header.Get("X-Forwarded-Email")

        // Store in context
        ctx := context.WithValue(r.Context(), "jwt_token", token)
        ctx = context.WithValue(ctx, "user_id", userID)
        ctx = context.WithValue(ctx, "email", email)

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

**GraphQL Resolver:**
```go
func (r *Resolver) ListRepositories(ctx context.Context) (*RepositoryConnection, error) {
    // Token already validated by Istio
    // Just use it!
    repos, err := r.giteaClient.ListRepositories(ctx, 1, 10)
    return repos, err
}
```

### Step 5: Gitea OAuth2 Validation

**gitea-service passes token to Gitea:**
```go
func (c *OAuth2Client) makeRequest(ctx context.Context, method, path string, body interface{}) (*http.Request, error) {
    // Create request
    req, _ := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)

    // Extract token from context (set by middleware)
    token := ctx.Value("jwt_token").(string)

    // Pass same token to Gitea
    req.Header.Set("Authorization", "Bearer " + token)

    return req, nil
}
```

**Gitea validates JWT:**
1. Receives: `Authorization: Bearer eyJhbGc...`
2. Validates JWT using Keycloak JWKS endpoint
3. Extracts user info from claims (`preferred_username`, `email`)
4. Checks user permissions in Gitea database
5. Returns repository data if authorized

## JWT Token Structure

**Decoded JWT (header):**
```json
{
  "alg": "RS256",
  "typ": "JWT",
  "kid": "abc123..."
}
```

**Decoded JWT (payload):**
```json
{
  "exp": 1704484800,
  "iat": 1704484000,
  "jti": "uuid...",
  "iss": "http://keycloak.auth-system:8080/realms/devplatform",
  "aud": ["gitea-service", "ldap-manager-service"],
  "sub": "user-uuid",
  "typ": "Bearer",
  "azp": "gitea-service",
  "preferred_username": "john.doe",
  "email": "john.doe@devplatform.local",
  "email_verified": true,
  "name": "John Doe",
  "given_name": "John",
  "family_name": "Doe",
  "realm_access": {
    "roles": ["developer", "user"]
  },
  "resource_access": {
    "gitea-service": {
      "roles": ["read", "write"]
    }
  }
}
```

## Benefits of Token Passthrough

### ✅ Single Token
- User logs in once
- One token for all services
- No token exchange complexity

### ✅ Istio Does Validation
- JWT validated at gateway
- Services don't need JWT libraries
- Centralized authentication logic

### ✅ No Service-Level Secrets
- Services don't store client secrets
- No token generation code
- Simpler service implementation

### ✅ True OAuth2
- Standard OAuth2/OIDC flow
- Gitea is a proper resource server
- Keycloak is the authorization server

### ✅ Defense in Depth
- Layer 1: Istio validates at gateway
- Layer 2: Gitea validates again
- Layer 3: Gitea checks permissions

## Security Considerations

### Token Storage
- **Client-side**: Store in httpOnly cookie or secure storage
- **Never** store in localStorage (XSS vulnerability)
- Use refresh tokens to renew expired access tokens

### Token Expiration
- Access tokens: 15 minutes (short-lived)
- Refresh tokens: 30 days
- Client must handle token refresh

### HTTPS in Production
- Enable TLS in Keycloak: `KC_HTTPS_ENABLED=true`
- Use cert-manager for certificates
- Configure Istio for mutual TLS

### RBAC with Istio
```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
  name: gitea-service-rbac
spec:
  action: ALLOW
  rules:
  - when:
    - key: request.auth.claims[realm_access.roles]
      values: ["developer", "admin"]
```

## Testing the Flow

### 1. Get Token
```bash
TOKEN=$(curl -s -X POST http://keycloak.localhost/realms/devplatform/protocol/openid-connect/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password" \
  -d "client_id=gitea-service" \
  -d "client_secret=<secret>" \
  -d "username=john.doe" \
  -d "password=password123" | jq -r '.access_token')

echo $TOKEN
```

### 2. Use Token in GraphQL
```bash
curl -X POST http://gitea-service.localhost/graphql \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query":"{ listRepositories { items { name } } }"}'
```

### 3. Decode JWT (for debugging)
```bash
echo $TOKEN | cut -d'.' -f2 | base64 -d | jq
```

### 4. Verify Istio Headers
```bash
# Deploy a debug pod with netcat
kubectl run -it debug --image=nicolaka/netshoot --rm -- bash

# Inside pod, listen and make request
nc -l -p 8080

# From another terminal, forward request
kubectl port-forward debug 8080:8080

# Make GraphQL request and see headers Istio adds
```

## Troubleshooting

### Token Validation Fails

**Error:** `401 Unauthorized` from Istio

**Check:**
```bash
# Verify Keycloak JWKS is accessible
curl http://keycloak.auth-system:8080/realms/devplatform/protocol/openid-connect/certs

# Check RequestAuthentication
kubectl get requestauthentication -n istio-system keycloak-jwt -o yaml

# Check Istio logs
kubectl logs -n istio-system -l app=istiod | grep jwt
```

### Token Not Passed to Gitea

**Error:** Gitea returns `401 Unauthorized`

**Check:**
```bash
# Verify token in context
# Add logging in middleware:
c.logger.WithField("token", token).Debug("Token from context")

# Check Gitea OAuth2 configuration
kubectl exec -it gitea-0 -n dev-platform -- cat /data/gitea/conf/app.ini | grep oauth2

# Test Gitea directly
curl -H "Authorization: Bearer $TOKEN" http://gitea.dev-platform:3000/api/v1/user
```

### User Not Found in Gitea

**Error:** `404 User not found`

**Solution:**
- Ensure Keycloak LDAP federation is working
- Check `ENABLE_AUTO_REGISTRATION = true` in Gitea config
- User must login via Gitea UI first to create account
- Or use Gitea API to pre-create users

## Migration from Dual-Token

### Before (Dual-Token):
```go
// Old code - DO NOT USE
func (r *Resolver) ListRepositories(ctx context.Context) {
    // Get LDAP token
    ldapToken := getLDAPToken(ctx)

    // Validate with LDAP
    user := ldapClient.ValidateToken(ldapToken)

    // Generate Gitea token
    giteaToken := generateGiteaToken(user)

    // Call Gitea
    repos := giteaClient.ListRepos(giteaToken)
}
```

### After (Token Passthrough):
```go
// New code - CLEAN!
func (r *Resolver) ListRepositories(ctx context.Context) {
    // Token already validated by Istio
    // Just call Gitea - token passed automatically
    repos, err := r.giteaClient.ListRepositories(ctx, 1, 10)
    return repos, err
}
```

## References

- [RFC 6749 - OAuth 2.0](https://tools.ietf.org/html/rfc6749)
- [RFC 7523 - JWT Bearer Token](https://tools.ietf.org/html/rfc7523)
- [Istio JWT Authentication](https://istio.io/latest/docs/tasks/security/authentication/authn-policy/)
- [Keycloak Documentation](https://www.keycloak.org/documentation)
- [Gitea OAuth2 Provider](https://docs.gitea.io/en-us/oauth2-provider/)
