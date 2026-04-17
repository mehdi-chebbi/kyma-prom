# OAuth2 Token Passthrough Implementation

## 🎯 What We Built

A **clean, single-token OAuth2 architecture** with Keycloak, Istio service mesh, and Gitea integration.

### ✅ No More Complexity

**Before (Messy):**
```
User → Service (LDAP token) → Service (Gitea token) → Gitea
       ❌ Two tokens
       ❌ Dual validation
       ❌ Token generation logic
```

**After (Clean):**
```
User → Keycloak (one JWT) → Istio (validates) → Service → Gitea
       ✅ One token
       ✅ Istio validates
       ✅ Token passthrough
```

## 📁 What Was Created

### 1. **Keycloak OAuth2 Server** (k8s/auth/)
```
auth/
├── 01-namespace.yaml              # auth-system namespace
├── 02-postgres.yaml               # Shared PostgreSQL (10Gi)
├── 03-memcached.yaml              # Session cache (3 replicas)
├── 04-keycloak.yaml               # Keycloak HA (2 replicas)
├── 05-keycloak-ingress.yaml       # Traefik ingress
├── 06-istio-auth.yaml             # JWT validation policies
└── 07-keycloak-ldap-config.yaml   # Auto LDAP federation
```

**Purpose:** Authorization server that issues JWT tokens

### 2. **Gitea Resource Server** (k8s/dev-platform/)
```
dev-platform/
├── gitea-deployment.yaml          # Gitea + PostgreSQL DB
└── gitea-oauth2-config.yaml       # OAuth2 client config
```

**Purpose:** Resource server that validates JWT tokens from Keycloak

### 3. **gitea-service** (Token Passthrough)
```
gitea-service/internal/
├── auth/
│   └── middleware.go              # Extract Istio headers
└── gitea/
    └── client_oauth2.go           # Pass token to Gitea
```

**Purpose:** GraphQL API that passes tokens (no generation)

### 4. **Documentation**
```
├── KEYCLOAK.md                    # Full Keycloak guide
├── OAUTH2_IMPLEMENTATION.md       # This file
└── gitea-service/OAUTH2_FLOW.md   # Technical flow details
```

### 5. **Deployment Scripts**
```
k8s/
├── auth/deploy-keycloak.bat       # Deploy Keycloak stack
└── deploy-oauth2-stack.bat        # Deploy everything
```

## 🚀 Quick Start

### Deploy Everything

```bash
cd k8s
deploy-oauth2-stack.bat
```

This deploys:
1. **PostgreSQL** (shared by Keycloak + Gitea)
2. **Memcached** (Keycloak sessions)
3. **Keycloak** (OAuth2 server)
4. **Istio Policies** (JWT validation)
5. **Gitea** (with OAuth2 config)

### Get Access Token

```bash
curl -X POST http://keycloak.localhost/realms/devplatform/protocol/openid-connect/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password" \
  -d "client_id=gitea-service" \
  -d "client_secret=<from-secret>" \
  -d "username=john.doe" \
  -d "password=password123"
```

**Returns:**
```json
{
  "access_token": "eyJhbGc...",
  "expires_in": 900,
  "refresh_token": "eyJhbGc...",
  "token_type": "Bearer"
}
```

### Use Token in GraphQL

```bash
curl -X POST http://gitea-service.localhost/graphql \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{"query":"{ listRepositories { items { name } } }"}'
```

## 🔒 How It Works

### The Flow

```
1. User Login
   ├─ POST /realms/devplatform/protocol/openid-connect/token
   └─ Returns JWT token

2. GraphQL Request
   ├─ POST /graphql
   ├─ Header: Authorization: Bearer <token>
   └─ Istio validates JWT signature

3. Istio Gateway
   ├─ Validates JWT with Keycloak JWKS
   ├─ Checks expiration
   ├─ Adds headers: X-Forwarded-User, X-Forwarded-Email
   └─ Forwards to gitea-service

4. gitea-service
   ├─ Middleware extracts token from headers
   ├─ Stores in context
   └─ Passes same token to Gitea API

5. Gitea API
   ├─ Validates JWT with Keycloak JWKS
   ├─ Checks user permissions
   └─ Returns repository data
```

### Istio Configuration

**RequestAuthentication** (validates JWT):
```yaml
apiVersion: security.istio.io/v1beta1
kind: RequestAuthentication
metadata:
  name: keycloak-jwt
  namespace: istio-system
spec:
  jwtRules:
  - issuer: "http://keycloak.auth-system:8080/realms/devplatform"
    jwksUri: "http://keycloak.auth-system:8080/.../certs"
    forwardOriginalToken: true
```

**AuthorizationPolicy** (enforces RBAC):
```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
  name: require-jwt
  namespace: dev-platform
spec:
  action: ALLOW
  rules:
  - from:
    - source:
        requestPrincipals: ["*"]
```

### gitea-service Code

**Middleware** (extracts from Istio):
```go
func (m *Middleware) ExtractToken(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Token already validated by Istio
        token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
        userID := r.Header.Get("X-Forwarded-User")

        // Store in context
        ctx := context.WithValue(r.Context(), "jwt_token", token)
        ctx = context.WithValue(ctx, "user_id", userID)

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

**Client** (passes token):
```go
func (c *OAuth2Client) makeRequest(ctx context.Context, method, path string) {
    token := ctx.Value("jwt_token").(string)

    req.Header.Set("Authorization", "Bearer " + token)
    // Gitea validates this JWT with Keycloak
}
```

## 🎯 Key Benefits

### 1. **Single Token**
- User logs in once
- One JWT for all services
- No token exchange

### 2. **Istio Handles Auth**
- JWT validated at gateway
- Services don't need JWT libraries
- Declarative security policies

### 3. **Standard OAuth2**
- Keycloak = Authorization Server
- Gitea = Resource Server
- gitea-service = Client Application

### 4. **Resource Optimization**
- Shared PostgreSQL (Keycloak + Gitea)
- Memcached for distributed sessions
- Efficient resource usage

### 5. **Defense in Depth**
```
Layer 1: Istio Ingress → Validates JWT
Layer 2: Service Mesh → mTLS encryption
Layer 3: Gitea → Re-validates JWT
Layer 4: Application → Checks permissions
```

## 📊 Architecture Comparison

### Traditional Microservices
```
Service A → Validates JWT (code)
Service B → Validates JWT (code)
Service C → Validates JWT (code)
```
**Problem:** Duplicate validation logic

### With Service Mesh
```
Istio → Validates JWT once
  ├─ Service A → Trusts Istio
  ├─ Service B → Trusts Istio
  └─ Service C → Trusts Istio
```
**Benefit:** Centralized validation

## 🔧 Configuration Files

### Keycloak Realm: `devplatform`
```
Realm Settings:
├─ Login with email: ✅
├─ User registration: ❌ (LDAP only)
├─ Password reset: ✅
└─ Remember me: ✅

LDAP Federation:
├─ Connection: ldap://openldap.dev-platform:389
├─ Users DN: ou=users,dc=devplatform,dc=local
├─ Import users: ✅
├─ Edit mode: READ_ONLY
└─ Sync: Every 5 minutes

Clients:
├─ gitea-service (confidential)
│  ├─ Client protocol: openid-connect
│  ├─ Access type: confidential
│  ├─ Standard flow: ✅
│  ├─ Direct access grants: ✅
│  └─ Service accounts: ✅
└─ ldap-manager-service (confidential)
   └─ [Same as above]
```

### Gitea OAuth2 Config
```ini
[oauth2]
ENABLED = true
JWT_SIGNING_ALGORITHM = RS256

[oauth2_client]
ENABLE_AUTO_REGISTRATION = true
USERNAME = preferred_username
EMAIL = email
UPDATE_AVATAR = true

[openid]
ENABLE_OPENID_SIGNIN = true
WHITELISTED_URIS = keycloak.auth-system.svc.cluster.local
```

## 🧪 Testing

### 1. **Test Token Acquisition**
```bash
TOKEN=$(curl -s -X POST http://keycloak.localhost/realms/devplatform/protocol/openid-connect/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password" \
  -d "client_id=gitea-service" \
  -d "client_secret=$(kubectl get secret keycloak-client-secrets -n auth-system -o jsonpath='{.data.gitea-service-secret}' | base64 -d)" \
  -d "username=john.doe" \
  -d "password=password123" | jq -r '.access_token')

echo "Token: $TOKEN"
```

### 2. **Test GraphQL**
```bash
curl -X POST http://gitea-service.localhost/graphql \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query":"{ listRepositories { items { name owner { username } } } }"}'
```

### 3. **Decode JWT** (debug)
```bash
# Decode header
echo $TOKEN | cut -d'.' -f1 | base64 -d | jq

# Decode payload
echo $TOKEN | cut -d'.' -f2 | base64 -d | jq
```

### 4. **Test Istio Validation**
```bash
# Try without token (should fail)
curl -X POST http://gitea-service.localhost/graphql \
  -H "Content-Type: application/json" \
  -d '{"query":"{ health }"}'

# Expected: 401 Unauthorized (Istio blocks)
```

## 🐛 Troubleshooting

### Token Validation Fails

**Symptom:** `401 Unauthorized` from Istio

**Check:**
```bash
# Verify JWKS endpoint
curl http://keycloak.auth-system:8080/realms/devplatform/protocol/openid-connect/certs

# Check Istio config
kubectl get requestauthentication -n istio-system keycloak-jwt -o yaml

# Check logs
kubectl logs -n istio-system -l app=istiod | grep jwt
```

### Gitea Returns 401

**Symptom:** Token passes Istio but Gitea rejects

**Check:**
```bash
# Verify Gitea OAuth2 config
kubectl exec -it $(kubectl get pod -n dev-platform -l app=gitea -o name) -n dev-platform -- \
  cat /data/gitea/conf/app.ini | grep -A 10 oauth2

# Test Gitea directly
curl -H "Authorization: Bearer $TOKEN" http://gitea.dev-platform:3000/api/v1/user
```

### User Not Auto-Registered

**Symptom:** `404 User not found` from Gitea

**Solution:**
1. Check `ENABLE_AUTO_REGISTRATION = true` in Gitea config
2. User must login via Gitea web UI first
3. Or pre-create users via API

## 📚 Documentation

- **KEYCLOAK.md**: Complete Keycloak deployment guide
- **OAUTH2_FLOW.md**: Technical token flow details
- **CLAUDE.md**: Updated with OAuth2 architecture

## 🎉 What This Achieves

### Before
```
❌ Two tokens (LDAP + Gitea)
❌ Complex token generation
❌ Service-level auth code
❌ Hard to test
❌ Security vulnerabilities
```

### After
```
✅ One token (Keycloak JWT)
✅ Token passthrough
✅ Istio validates
✅ Easy to test
✅ Standard OAuth2
```

## 🚀 Next Steps

1. **Deploy:** Run `deploy-oauth2-stack.bat`
2. **Test:** Get token and make GraphQL requests
3. **Monitor:** Check Keycloak metrics and Istio telemetry
4. **Secure:** Enable TLS in production
5. **Scale:** Adjust Keycloak/Gitea replicas as needed

## 📞 Support

- Keycloak docs: https://www.keycloak.org/documentation
- Istio security: https://istio.io/latest/docs/tasks/security/
- Gitea OAuth2: https://docs.gitea.io/en-us/oauth2-provider/

---

**Clean. Simple. Standard OAuth2.** 🎯
