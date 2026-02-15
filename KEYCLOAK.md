# Keycloak OAuth2/OpenID Connect Authentication Service

## Overview

Keycloak is our centralized authentication and authorization service providing OAuth2/OpenID Connect for all microservices in the platform. It integrates with OpenLDAP as the user federation backend and uses Memcached for distributed session caching.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│               Istio Ingress Gateway                     │
│          (TLS Termination + Routing)                    │
└────────────────────┬────────────────────────────────────┘
                     │
        ┌────────────┴─────────────┐
        │                          │
        ▼                          ▼
┌───────────────────┐      ┌──────────────────┐
│   Keycloak UI     │      │  GraphQL Services │
│ (Auth Portal)     │      │  (gitea-service)  │
└────────┬──────────┘      └─────────┬─────────┘
         │                           │
         │  ┌────────────────────────┘
         │  │ JWT Token Validation
         │  │ (via Istio RequestAuthentication)
         ▼  ▼
┌─────────────────────────────────────┐
│          Keycloak Server            │
│  - OAuth2/OIDC Provider             │
│  - User Federation (LDAP)           │
│  - JWT Token Issuer                 │
│  - Session Management               │
└──────────┬──────────────┬───────────┘
           │              │
           ▼              ▼
  ┌────────────┐   ┌──────────────┐
  │  OpenLDAP  │   │  Memcached   │
  │ (Users DB) │   │ (Sessions)   │
  └────────────┘   └──────────────┘
```

## Technology Stack

- **Keycloak**: 23.0 (Latest stable)
- **Database**: PostgreSQL 15 (for Keycloak metadata)
- **Session Cache**: Memcached 1.6
- **User Storage**: OpenLDAP (LDAP Federation)
- **Service Mesh**: Istio for JWT validation
- **Container Orchestration**: Kubernetes

## Features

### 1. **Single Sign-On (SSO)**
- One login for all platform services
- JWT tokens validated at Istio gateway level
- No need for service-specific authentication

### 2. **LDAP Federation**
- Keycloak syncs users from OpenLDAP
- Read-only LDAP connection
- Automatic user import on first login
- Password validation against LDAP

### 3. **OAuth2/OpenID Connect Flows**
- **Authorization Code Flow**: For web applications
- **Client Credentials Flow**: For service-to-service communication
- **Refresh Token Flow**: For token renewal without re-login
- **Resource Owner Password Flow**: For trusted clients (GraphQL mutations)

### 4. **Session Management with Memcached**
- Distributed session caching
- High availability
- Fast session lookup
- Reduced database load

### 5. **Istio Integration**
- JWT validation at ingress gateway
- No authentication logic in services
- RBAC enforcement via AuthorizationPolicy
- Automatic token propagation

## Directory Structure

```
k8s/
├── auth/
│   ├── 01-namespace.yaml           # Namespace: auth-system
│   ├── 02-postgres.yaml            # PostgreSQL for Keycloak
│   ├── 03-memcached.yaml           # Memcached for sessions
│   ├── 04-keycloak.yaml            # Keycloak StatefulSet
│   ├── 05-keycloak-service.yaml    # Service + IngressRoute
│   └── 06-istio-auth.yaml          # Istio JWT validation
└── dev-platform/
    └── 07-service-auth-policies.yaml  # Per-service AuthZ policies
```

## Deployment Components

### 1. PostgreSQL for Keycloak

**Purpose**: Store Keycloak configuration, clients, roles, and realm data

**Specifications**:
- StatefulSet with 1 replica
- Persistent Volume: 10Gi
- Database: `keycloak`
- Resources: 256Mi memory, 250m CPU

**Environment Variables**:
```yaml
POSTGRES_DB: keycloak
POSTGRES_USER: keycloak
POSTGRES_PASSWORD: <from secret>
```

### 2. Memcached for Sessions

**Purpose**: Distributed session caching for high availability

**Specifications**:
- Deployment with 3 replicas
- Memory: 256Mi per pod
- Port: 11211
- No persistence (cache only)

**Configuration**:
```yaml
Command: memcached -m 256 -c 1024 -I 5m
```

**Features**:
- LRU eviction
- 256MB cache size per pod
- Max 1024 concurrent connections
- 5MB max item size

### 3. Keycloak Server

**Purpose**: OAuth2/OIDC provider and authentication server

**Specifications**:
- StatefulSet with 2 replicas (HA)
- Database: PostgreSQL
- Cache: Memcached (Infinispan distributed cache)
- Persistent Volume: 5Gi for themes/extensions

**Environment Variables**:
```yaml
KC_DB: postgres
KC_DB_URL: jdbc:postgresql://postgres:5432/keycloak
KC_DB_USERNAME: keycloak
KC_DB_PASSWORD: <from secret>
KC_CACHE: ispn
KC_CACHE_STACK: kubernetes
KEYCLOAK_ADMIN: admin
KEYCLOAK_ADMIN_PASSWORD: <from secret>
KC_HOSTNAME: keycloak.localhost
KC_PROXY: edge
KC_HTTP_ENABLED: true
```

**Startup Command**:
```bash
/opt/keycloak/bin/kc.sh start --auto-build \
  --http-enabled=true \
  --hostname-strict=false \
  --proxy=edge \
  --cache=ispn \
  --cache-stack=kubernetes
```

**Ports**:
- 8080: HTTP (behind Istio)
- 9000: Metrics (Prometheus)

### 4. Istio JWT Validation

**RequestAuthentication**: Validates JWT tokens from Keycloak

```yaml
apiVersion: security.istio.io/v1beta1
kind: RequestAuthentication
metadata:
  name: keycloak-jwt
  namespace: istio-system
spec:
  selector:
    matchLabels:
      istio: ingressgateway
  jwtRules:
  - issuer: "http://keycloak.auth-system.svc.cluster.local:8080/realms/devplatform"
    jwksUri: "http://keycloak.auth-system.svc.cluster.local:8080/realms/devplatform/protocol/openid-connect/certs"
    forwardOriginalToken: true
```

**AuthorizationPolicy**: Require valid JWT for protected services

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
    to:
    - operation:
        paths: ["/graphql"]
```

## Keycloak Configuration

### Realm Setup

**Realm Name**: `devplatform`

**Realm Settings**:
- Login: Username or Email
- User Registration: Disabled (LDAP users only)
- Forgot Password: Enabled
- Remember Me: Enabled
- Email as username: Enabled

### LDAP Federation

**User Federation**: OpenLDAP

**Connection Settings**:
```
Connection URL: ldap://openldap.dev-platform.svc.cluster.local:389
Bind DN: cn=admin,dc=devplatform,dc=local
Bind Credential: <admin password>
Users DN: ou=users,dc=devplatform,dc=local
```

**LDAP Mappers**:
| Mapper Name | LDAP Attribute | User Attribute |
|-------------|----------------|----------------|
| username | uid | username |
| email | mail | email |
| first name | givenName | firstName |
| last name | sn | lastName |
| department | departmentNumber | department |
| repositories | githubRepository | repositories |

**Sync Settings**:
- Import Users: ON
- Edit Mode: READ_ONLY
- Sync Registrations: OFF
- Vendor: Other (OpenLDAP)
- Batch Size: 100
- Periodic Full Sync: Enabled (1 hour)
- Periodic Changed Users Sync: Enabled (5 minutes)

### Clients Configuration

#### 1. **gitea-service** (GraphQL Backend)

**Client ID**: `gitea-service`
**Client Protocol**: openid-connect
**Access Type**: confidential

**Settings**:
```yaml
Root URL: http://gitea-service.dev-platform.svc.cluster.local:8080
Valid Redirect URIs:
  - http://gitea-service.dev-platform.svc.cluster.local:8080/*
  - http://localhost:8080/*
Web Origins: *
```

**Capabilities**:
- Standard Flow: Enabled
- Direct Access Grants: Enabled (for password flow in mutations)
- Service Accounts: Enabled (for service-to-service)

**Scopes**:
- openid
- profile
- email
- roles

**Token Settings**:
- Access Token Lifespan: 15 minutes
- Refresh Token Lifespan: 30 days
- Client Session Idle: 30 minutes
- Client Session Max: 10 hours

#### 2. **ldap-manager-service**

**Client ID**: `ldap-manager-service`
**Access Type**: confidential

Same settings as gitea-service.

#### 3. **frontend-app** (Future Web UI)

**Client ID**: `frontend-app`
**Access Type**: public

**Settings**:
- Standard Flow: Enabled
- Valid Redirect URIs: http://localhost:3000/*

## Authentication Flows

### 1. **User Login via GraphQL**

**GraphQL Mutation**:
```graphql
mutation Login {
  login(username: "john.doe", password: "password123") {
    accessToken
    refreshToken
    expiresIn
    tokenType
    user {
      uid
      email
      department
    }
  }
}
```

**Backend Flow**:
```
1. Client → GraphQL login mutation
2. GraphQL → Keycloak Token Endpoint (Resource Owner Password Flow)
   POST /realms/devplatform/protocol/openid-connect/token
   Body: {
     grant_type: "password",
     client_id: "gitea-service",
     client_secret: "<secret>",
     username: "john.doe",
     password: "password123"
   }
3. Keycloak → LDAP (validate password)
4. Keycloak → Issues JWT token
5. GraphQL → Returns token to client
6. Client stores token (localStorage, secure cookie)
```

### 2. **Authenticated GraphQL Request**

**Request**:
```bash
curl -X POST http://gitea-service.localhost/graphql \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{"query":"{ listRepositories { items { name } } }"}'
```

**Flow**:
```
1. Client → Istio Ingress (with JWT in Authorization header)
2. Istio → Validates JWT signature using Keycloak JWKS
3. Istio → Extracts claims (uid, email, roles)
4. Istio → Forwards request to service with claims in headers
5. Service → Trusts validated claims from Istio
6. Service → Returns data
```

### 3. **Token Refresh**

**GraphQL Mutation**:
```graphql
mutation Refresh {
  refreshToken(refreshToken: "<refresh_token>") {
    accessToken
    refreshToken
    expiresIn
  }
}
```

**Backend Flow**:
```
POST /realms/devplatform/protocol/openid-connect/token
Body: {
  grant_type: "refresh_token",
  client_id: "gitea-service",
  client_secret: "<secret>",
  refresh_token: "<refresh_token>"
}
```

### 4. **Service-to-Service Authentication**

**Example**: Gitea-service calling LDAP-manager-service

```go
// Get client credentials token
func (c *Client) getServiceToken() (string, error) {
    data := url.Values{}
    data.Set("grant_type", "client_credentials")
    data.Set("client_id", "gitea-service")
    data.Set("client_secret", c.config.ClientSecret)

    resp, err := http.Post(
        "http://keycloak.auth-system:8080/realms/devplatform/protocol/openid-connect/token",
        "application/x-www-form-urlencoded",
        strings.NewReader(data.Encode()),
    )

    // Parse token response
    var tokenResp TokenResponse
    json.NewDecoder(resp.Body).Decode(&tokenResp)

    return tokenResp.AccessToken, nil
}

// Call LDAP Manager with token
func (c *Client) GetUser(uid string) (*User, error) {
    token, _ := c.getServiceToken()

    req, _ := http.NewRequest("POST", "http://ldap-manager:8080/graphql", body)
    req.Header.Set("Authorization", "Bearer " + token)

    // Make request
}
```

## JWT Token Structure

**Example JWT Claims**:
```json
{
  "exp": 1704484800,
  "iat": 1704484000,
  "jti": "a5f3d8c2-4b1e-4f7a-9c2d-3e8f5a1b6c9d",
  "iss": "http://keycloak.auth-system:8080/realms/devplatform",
  "aud": ["gitea-service", "ldap-manager-service"],
  "sub": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "typ": "Bearer",
  "azp": "gitea-service",
  "session_state": "1a2b3c4d-5e6f-7g8h-9i0j-k1l2m3n4o5p6",
  "preferred_username": "john.doe",
  "email": "john.doe@devplatform.local",
  "email_verified": true,
  "name": "John Doe",
  "given_name": "John",
  "family_name": "Doe",
  "department": "engineering",
  "repositories": [
    "https://github.com/devplatform/backend",
    "https://github.com/devplatform/frontend"
  ],
  "realm_access": {
    "roles": ["user", "developer"]
  },
  "resource_access": {
    "gitea-service": {
      "roles": ["read", "write"]
    }
  },
  "scope": "openid profile email"
}
```

## Monitoring and Observability

### Keycloak Metrics

**Endpoint**: `http://keycloak:9000/metrics`

**Key Metrics**:
- `keycloak_logins_total`: Total login attempts
- `keycloak_failed_login_attempts_total`: Failed logins
- `keycloak_user_sessions_total`: Active user sessions
- `keycloak_client_sessions_total`: Active client sessions
- `keycloak_request_duration_seconds`: Request latency

### Logs

**Format**: JSON structured logs

**Example Log Entry**:
```json
{
  "timestamp": "2024-01-05T10:30:00.000Z",
  "level": "INFO",
  "logger": "org.keycloak.events",
  "message": "LOGIN",
  "type": "LOGIN",
  "realmId": "devplatform",
  "clientId": "gitea-service",
  "userId": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "ipAddress": "10.42.1.15",
  "username": "john.doe"
}
```

### Health Checks

**Liveness Probe**:
```yaml
httpGet:
  path: /health/live
  port: 8080
initialDelaySeconds: 120
periodSeconds: 30
```

**Readiness Probe**:
```yaml
httpGet:
  path: /health/ready
  port: 8080
initialDelaySeconds: 60
periodSeconds: 10
```

## Security Considerations

### 1. **Secrets Management**

All sensitive data stored in Kubernetes Secrets:
- `keycloak-db-secret`: PostgreSQL credentials
- `keycloak-admin-secret`: Keycloak admin password
- `keycloak-client-secrets`: Client secrets for services
- `ldap-bind-secret`: LDAP bind credentials

### 2. **Network Policies**

Restrict traffic to Keycloak:
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: keycloak-network-policy
  namespace: auth-system
spec:
  podSelector:
    matchLabels:
      app: keycloak
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: istio-system
    - namespaceSelector:
        matchLabels:
          name: dev-platform
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: postgres
    ports:
    - protocol: TCP
      port: 5432
  - to:
    - namespaceSelector:
        matchLabels:
          name: dev-platform
    - podSelector:
        matchLabels:
          app: openldap
    ports:
    - protocol: TCP
      port: 389
```

### 3. **TLS Configuration**

**Production Setup**:
- Enable TLS in Keycloak: `KC_HTTPS_ENABLED=true`
- Use cert-manager for certificates
- Configure Istio for mutual TLS

### 4. **Token Security**

- Short-lived access tokens (15 min)
- Refresh tokens stored securely
- Token revocation on logout
- Audience validation in services

## High Availability

### Keycloak HA Setup

**Requirements**:
- Minimum 2 replicas
- PostgreSQL with replication
- Memcached cluster (3+ nodes)
- Pod Anti-Affinity

**Configuration**:
```yaml
affinity:
  podAntiAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
    - labelSelector:
        matchExpressions:
        - key: app
          operator: In
          values:
          - keycloak
      topologyKey: kubernetes.io/hostname
```

### Session Replication

Keycloak uses Infinispan with Memcached backend:
```yaml
KC_CACHE: ispn
KC_CACHE_STACK: kubernetes
JAVA_OPTS: "-Djgroups.dns.query=keycloak-headless.auth-system.svc.cluster.local"
```

### Database HA

PostgreSQL with streaming replication:
- Primary: Read/write
- Standby: Read-only replica
- Automatic failover with Patroni

## Disaster Recovery

### Backup Strategy

**What to Backup**:
1. PostgreSQL database (Keycloak config)
2. Realm export (JSON)
3. Client secrets

**Backup Schedule**:
- Daily automated backups
- Retention: 30 days
- Test restore monthly

**Export Realm**:
```bash
kubectl exec -it keycloak-0 -n auth-system -- \
  /opt/keycloak/bin/kc.sh export \
  --dir /tmp/export \
  --realm devplatform
```

### Recovery Procedure

1. Restore PostgreSQL from backup
2. Deploy Keycloak pods
3. Import realm configuration
4. Update client secrets in services
5. Verify LDAP federation

## Testing

### 1. **Token Acquisition**

```bash
# Login
curl -X POST http://keycloak.localhost/realms/devplatform/protocol/openid-connect/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password" \
  -d "client_id=gitea-service" \
  -d "client_secret=<secret>" \
  -d "username=john.doe" \
  -d "password=password123"
```

### 2. **Token Validation**

```bash
# Introspect token
curl -X POST http://keycloak.localhost/realms/devplatform/protocol/openid-connect/token/introspect \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "token=<access_token>" \
  -d "client_id=gitea-service" \
  -d "client_secret=<secret>"
```

### 3. **GraphQL with Token**

```bash
TOKEN="<access_token>"

curl -X POST http://gitea-service.localhost/graphql \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query":"{ me { uid email department } }"}'
```

## Troubleshooting

### Common Issues

**1. JWT Validation Fails in Istio**
```bash
# Check RequestAuthentication
kubectl get requestauthentication -n istio-system

# Verify JWKS endpoint
curl http://keycloak.auth-system:8080/realms/devplatform/protocol/openid-connect/certs

# Check Istio logs
kubectl logs -n istio-system -l app=istiod
```

**2. Keycloak Can't Connect to LDAP**
```bash
# Test LDAP connection
kubectl exec -it keycloak-0 -n auth-system -- bash
ldapsearch -x -H ldap://openldap.dev-platform:389 \
  -D "cn=admin,dc=devplatform,dc=local" \
  -w "<password>" \
  -b "ou=users,dc=devplatform,dc=local"
```

**3. Session Not Persisting**
```bash
# Check Memcached
kubectl get pods -n auth-system -l app=memcached
kubectl logs -n auth-system -l app=memcached

# Test Memcached connection
telnet memcached.auth-system 11211
```

## Performance Tuning

### Keycloak JVM Settings

```yaml
env:
- name: JAVA_OPTS
  value: >-
    -Xms512m
    -Xmx1024m
    -XX:MetaspaceSize=96M
    -XX:MaxMetaspaceSize=256m
    -Djava.net.preferIPv4Stack=true
    -Djboss.modules.system.pkgs=org.jboss.byteman
```

### Database Connection Pool

```yaml
KC_DB_POOL_INITIAL_SIZE: "10"
KC_DB_POOL_MIN_SIZE: "10"
KC_DB_POOL_MAX_SIZE: "50"
```

### Memcached Tuning

- Memory per pod: 256Mi - 512Mi
- Max connections: 1024
- Eviction policy: LRU

## Migration Path

### Current State → Keycloak

**Phase 1: Deploy Keycloak**
1. Deploy PostgreSQL
2. Deploy Memcached
3. Deploy Keycloak
4. Configure LDAP federation
5. Create realm and clients

**Phase 2: Update Services**
1. Add Keycloak client library
2. Implement login/refresh mutations
3. Remove dual-token logic
4. Use Keycloak tokens only

**Phase 3: Enable Istio Validation**
1. Deploy RequestAuthentication
2. Deploy AuthorizationPolicy
3. Test end-to-end flow
4. Remove service-level JWT validation (Istio handles it)

**Phase 4: Cleanup**
1. Remove old LDAP JWT generation
2. Simplify GraphQL auth resolvers
3. Update documentation

## References

- [Keycloak Documentation](https://www.keycloak.org/documentation)
- [Istio JWT Authentication](https://istio.io/latest/docs/tasks/security/authentication/authn-policy/)
- [Memcached Best Practices](https://github.com/memcached/memcached/wiki)
- [OAuth2 RFC 6749](https://tools.ietf.org/html/rfc6749)
- [OpenID Connect Core](https://openid.net/specs/openid-connect-core-1_0.html)
