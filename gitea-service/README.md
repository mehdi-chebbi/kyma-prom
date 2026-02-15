# Gitea Service Microservice

A standalone microservice for managing Gitea repository access control based on LDAP user and department attributes.

## Overview

The Gitea Service is a GraphQL microservice that:
- Integrates with Gitea REST API to list and manage repositories
- Communicates with LDAP Manager service to fetch user and department data
- Filters repositories based on user's personal `githubRepository` attribute and department repositories
- Provides repository search and statistics
- Uses JWT authentication (shared secret with LDAP Manager)

## Architecture

```
┌─────────────────────────────────────────────────┐
│         Frontend (GraphQL Client)               │
└──────────────┬──────────────────────────────────┘
               │ JWT Token
               ▼
┌─────────────────────────────────────────────────┐
│         Gitea Service (Port 8081)               │
│  - GraphQL API                                  │
│  - JWT validation                               │
│  - Repository filtering                         │
└────────┬────────────────────────────────┬───────┘
         │                                │
         │ HTTP GraphQL                   │ Gitea REST API
         ▼                                ▼
┌──────────────────────┐      ┌──────────────────────┐
│  LDAP Manager Service│      │  Gitea Server        │
│  (Port 8080)         │      │  (Port 3000)         │
│  - User data         │      │  - Repositories      │
│  - Department data   │      │  - Repository search │
└──────────────────────┘      └──────────────────────┘
```

## Features

- **Repository Access Control**: Users see only repositories assigned to them personally or through their department
- **GraphQL API**: Clean, type-safe API for frontend integration
- **Inter-service Communication**: Fetches user/department data from LDAP Manager via GraphQL
- **JWT Authentication**: Shared secret with LDAP Manager for seamless authentication
- **Prometheus Metrics**: Metrics exposed on port 9091
- **Health Checks**: Liveness and readiness probes for Kubernetes
- **High Availability**: 2+ replicas with pod anti-affinity
- **Auto-scaling**: HPA based on CPU and memory

## Prerequisites

- Kubernetes cluster with Traefik ingress controller
- LDAP Manager service deployed and accessible
- Gitea server deployed and accessible
- JWT secret shared with LDAP Manager

## Configuration

### Environment Variables

Configure via ConfigMap (`k8s/01-configmap.yaml`):

| Variable | Description | Example |
|----------|-------------|---------|
| `GITEA_URL` | Gitea server URL | `http://gitea.dev-platform.svc.cluster.local:3000` |
| `LDAP_MANAGER_URL` | LDAP Manager service URL | `http://ldap-manager.dev-platform.svc.cluster.local:8080` |
| `PORT` | HTTP server port | `8081` |
| `METRICS_PORT` | Prometheus metrics port | `9091` |
| `ENVIRONMENT` | Runtime environment | `production` |
| `LOG_LEVEL` | Logging level | `info` |
| `HTTP_CLIENT_TIMEOUT` | HTTP client timeout | `30s` |
| `JWT_EXPIRATION` | JWT token expiration | `24h` |
| `CORS_ORIGINS` | Allowed CORS origins | `*` |
| `SHUTDOWN_TIMEOUT` | Graceful shutdown timeout | `30` |

### Secrets

Configure via Secret (`k8s/02-secret.yaml`):

| Secret | Description | How to Generate |
|--------|-------------|-----------------|
| `GITEA_TOKEN` | Gitea admin token | Generate from Gitea UI: Settings → Applications → Generate New Token |
| `JWT_SECRET` | JWT signing secret | **MUST match LDAP Manager's JWT_SECRET** |

**IMPORTANT**: The `JWT_SECRET` must be identical to the one used by LDAP Manager for authentication to work.

## Deployment

### Step 1: Build Docker Image

```bash
cd gitea-service

# Build the image
docker build -t gitea-service:latest .

# Tag for your registry (if using one)
docker tag gitea-service:latest your-registry/gitea-service:latest
docker push your-registry/gitea-service:latest
```

### Step 2: Update Secrets

```bash
# Generate Gitea admin token from Gitea UI first
# Then create the secret

kubectl create secret generic gitea-service-secret \
  --from-literal=GITEA_TOKEN=your-gitea-admin-token \
  --from-literal=JWT_SECRET=your-jwt-secret-matching-ldap-manager \
  -n dev-platform
```

Or apply the manifest after editing:

```bash
# Edit k8s/02-secret.yaml with your values
kubectl apply -f k8s/02-secret.yaml
```

### Step 3: Deploy to Kubernetes

```bash
# Apply all manifests
kubectl apply -f k8s/

# Or apply individually
kubectl apply -f k8s/01-configmap.yaml
kubectl apply -f k8s/02-secret.yaml
kubectl apply -f k8s/03-deployment.yaml
kubectl apply -f k8s/04-service.yaml
kubectl apply -f k8s/05-ingress.yaml
```

### Step 4: Verify Deployment

```bash
# Check pod status
kubectl get pods -n dev-platform -l app=gitea-service

# Check logs
kubectl logs -f -l app=gitea-service -n dev-platform

# Check health
kubectl port-forward -n dev-platform svc/gitea-service 8081:80
curl http://localhost:8081/health
```

## GraphQL API

### Queries

#### 1. Get My Repositories

Fetches all repositories the authenticated user has access to (personal + department repos).

```graphql
query {
  myRepositories {
    id
    name
    fullName
    description
    private
    htmlUrl
    cloneUrl
    language
    stargazersCount
    forksCount
    size
    createdAt
    updatedAt
  }
}
```

#### 2. Get Specific Repository

Fetches a specific repository by owner/name, with access control check.

```graphql
query {
  repository(owner: "devplatform", name: "api-gateway") {
    id
    name
    fullName
    description
    private
    htmlUrl
    cloneUrl
  }
}
```

#### 3. Search Repositories

Search within accessible repositories.

```graphql
query {
  searchRepositories(query: "api") {
    id
    name
    fullName
    description
  }
}
```

#### 4. Get Repository Statistics

Get statistics about accessible repositories.

```graphql
query {
  repositoryStats {
    totalRepositories
    privateRepositories
    publicRepositories
    totalStars
    totalForks
    languageBreakdown {
      language
      count
    }
  }
}
```

#### 5. Health Check

Check service health and dependency status.

```graphql
query {
  health {
    status
    gitea
    ldapManager
    timestamp
  }
}
```

### Authentication

All queries (except `health`) require JWT authentication. Include the token in the Authorization header:

```bash
curl -X POST http://gitea-service.localhost/graphql \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{"query":"query { myRepositories { name fullName } }"}'
```

## Inter-Service Communication

The Gitea Service communicates with LDAP Manager to fetch user and department data:

```go
// Fetch user data from LDAP Manager
user, err := ldapClient.GetUser(ctx, uid, token)

// Fetch department data from LDAP Manager
dept, err := ldapClient.GetDepartment(ctx, departmentName, token)
```

The LDAP Manager service must be accessible at the URL specified in `LDAP_MANAGER_URL`.

## Repository Access Control

### How It Works

1. **User Authentication**: Frontend sends JWT token to Gitea Service
2. **Token Validation**: Gitea Service validates JWT using shared secret
3. **User Data Fetch**: Gitea Service fetches user data from LDAP Manager (includes `githubRepository` attribute)
4. **Department Data Fetch**: If user has a department, fetch department's `githubRepository` attribute
5. **Repository Filtering**: Gitea Service fetches all repos from Gitea and filters based on:
   - User's personal `githubRepository` values
   - User's department `githubRepository` values
6. **Response**: Returns only accessible repositories

### LDAP Attribute Structure

**User Entry:**
```ldif
dn: uid=john.doe,ou=users,dc=devplatform,dc=local
objectClass: inetOrgPerson
objectClass: posixAccount
objectClass: extensibleObject
uid: john.doe
cn: John Doe
mail: john.doe@company.com
departmentNumber: engineering
githubRepository: devplatform/user-dashboard
githubRepository: devplatform/mobile-app
```

**Department Entry:**
```ldif
dn: ou=engineering,ou=departments,dc=devplatform,dc=local
objectClass: organizationalUnit
objectClass: extensibleObject
ou: engineering
description: Engineering Department
githubRepository: devplatform/api-gateway
githubRepository: devplatform/frontend
githubRepository: devplatform/backend
```

**Result**: John Doe can access 5 repositories total:
- 2 personal repos (user-dashboard, mobile-app)
- 3 department repos (api-gateway, frontend, backend)

## Monitoring

### Prometheus Metrics

Metrics are exposed on port 9091 at `/metrics`:

```bash
kubectl port-forward -n dev-platform svc/gitea-service 9091:9091
curl http://localhost:9091/metrics
```

**Available Metrics:**
- `gitea_service_requests_total`: Total HTTP requests (by method, path, status)
- `gitea_service_request_duration_seconds`: Request duration histogram
- `gitea_operations_total`: Gitea operations count (by operation, status)

### Health Checks

**Liveness Probe**: `/health`
- Returns 200 if service is alive
- Used by Kubernetes to restart unhealthy pods

**Readiness Probe**: `/ready`
- Returns 200 if service is ready and dependencies are healthy
- Checks Gitea and LDAP Manager connectivity
- Used by Kubernetes to route traffic

## Troubleshooting

### Pod Not Starting

```bash
# Check pod events
kubectl describe pod -n dev-platform -l app=gitea-service

# Check logs
kubectl logs -n dev-platform -l app=gitea-service
```

Common issues:
- **Secret not found**: Ensure `gitea-service-secret` exists in `dev-platform` namespace
- **Image pull error**: Ensure Docker image is built and accessible
- **Configuration error**: Check ConfigMap values

### Authentication Failures

If queries return "unauthorized" errors:

1. **Check JWT Secret**: Ensure `JWT_SECRET` matches LDAP Manager
   ```bash
   kubectl get secret gitea-service-secret -n dev-platform -o jsonpath='{.data.JWT_SECRET}' | base64 -d
   kubectl get secret ldap-manager-secret -n dev-platform -o jsonpath='{.data.JWT_SECRET}' | base64 -d
   ```

2. **Test Token**: Get a token from LDAP Manager and test it
   ```bash
   # Login via LDAP Manager
   curl -X POST http://ldap-manager.localhost/graphql \
     -H "Content-Type: application/json" \
     -d '{"query":"mutation { login(uid: \"john.doe\", password: \"password123\") { token } }"}'

   # Use token with Gitea Service
   curl -X POST http://gitea-service.localhost/graphql \
     -H "Authorization: Bearer TOKEN_FROM_ABOVE" \
     -d '{"query":"query { myRepositories { name } }"}'
   ```

### No Repositories Returned

If authenticated user sees no repositories:

1. **Check LDAP Attributes**: Verify user has `githubRepository` attribute
   ```bash
   kubectl exec -it -n dev-platform deployment/ldap-manager -- sh
   ldapsearch -x -H ldap://openldap:389 -b "dc=devplatform,dc=local" -D "cn=admin,dc=devplatform,dc=local" -w admin "(uid=john.doe)" githubRepository
   ```

2. **Check Gitea Connectivity**: Verify Gitea is accessible
   ```bash
   kubectl exec -it -n dev-platform deployment/gitea-service -- sh
   wget -O- http://gitea.dev-platform.svc.cluster.local:3000/api/v1/repos/search
   ```

3. **Check Repository Names**: Ensure repository names in LDAP match Gitea repo full names (format: `owner/repo`)

### LDAP Manager Connection Issues

If readiness probe fails with "ldapManager: false":

1. **Check LDAP Manager Service**: Ensure it's running
   ```bash
   kubectl get pods -n dev-platform -l app=ldap-manager
   ```

2. **Test Connectivity**: Port forward and test
   ```bash
   kubectl port-forward -n dev-platform svc/ldap-manager 8080:80
   curl http://localhost:8080/health
   ```

3. **Check Configuration**: Verify `LDAP_MANAGER_URL` in ConfigMap
   ```bash
   kubectl get configmap gitea-service-config -n dev-platform -o yaml
   ```

## Development

### Local Development

```bash
# Install dependencies
go mod download

# Set environment variables
export GITEA_URL=http://localhost:3000
export GITEA_TOKEN=your-token
export LDAP_MANAGER_URL=http://localhost:8080
export JWT_SECRET=your-secret
export PORT=8081

# Run locally
go run cmd/server/main.go
```

### Testing

```bash
# Run tests
go test ./...

# With coverage
go test -cover ./...

# Verbose
go test -v ./...
```

## Security Considerations

1. **Change Default Secrets**: Always change the default Gitea token and JWT secret in production
2. **Use Strong JWT Secret**: Use a 32+ character random string
3. **Network Policies**: Consider adding Kubernetes NetworkPolicies to restrict traffic
4. **RBAC**: Ensure the ServiceAccount has minimal required permissions
5. **TLS**: Enable TLS for production deployments (update Ingress)
6. **Secret Management**: Use external secret management (Vault, Sealed Secrets, etc.) for production

## Performance Tuning

### Resource Limits

Adjust in `k8s/03-deployment.yaml`:

```yaml
resources:
  requests:
    memory: "128Mi"  # Minimum guaranteed
    cpu: "100m"
  limits:
    memory: "512Mi"  # Maximum allowed
    cpu: "500m"
```

### HPA Settings

Adjust in `k8s/05-ingress.yaml`:

```yaml
spec:
  minReplicas: 2      # Minimum replicas
  maxReplicas: 5      # Maximum replicas
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        averageUtilization: 70  # Scale at 70% CPU
```

### HTTP Client Timeout

Adjust in `k8s/01-configmap.yaml`:

```yaml
data:
  HTTP_CLIENT_TIMEOUT: "30s"  # Increase for slow networks
```

## Migration from Monolithic Backend

If migrating from the LDAP Manager backend with integrated Gitea:

1. **Deploy Gitea Service**: Follow deployment steps above
2. **Update Frontend**: Change GraphQL endpoint from LDAP Manager to Gitea Service for repository queries
3. **Test**: Verify authentication and repository access work
4. **Remove Old Code**: Optionally remove Gitea integration from LDAP Manager backend

See `MIGRATION.md` for detailed migration guide.

## License

Proprietary - DevPlatform

## Support

For issues or questions, contact the DevOps team or file an issue in the internal issue tracker.
