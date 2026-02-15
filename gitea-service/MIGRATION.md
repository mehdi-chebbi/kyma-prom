# Migration Guide: Gitea Integration to Microservice

This guide explains how to migrate from the monolithic LDAP Manager backend (with integrated Gitea) to the microservices architecture with a dedicated Gitea Service.

## Overview

### Before (Monolithic)

```
┌─────────────────────────────────────────────────┐
│         Frontend (GraphQL Client)               │
└──────────────────┬──────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────┐
│         LDAP Manager Backend                    │
│  - User Management                              │
│  - Department Management                        │
│  - Gitea Integration (embedded)                 │
│  - GraphQL API                                  │
└──────────┬─────────────────────────────┬────────┘
           │                             │
           │ LDAP Protocol               │ Gitea REST API
           ▼                             ▼
    ┌──────────┐                  ┌──────────┐
    │ OpenLDAP │                  │  Gitea   │
    └──────────┘                  └──────────┘
```

### After (Microservices)

```
┌─────────────────────────────────────────────────┐
│         Frontend (GraphQL Client)               │
└──────┬──────────────────────────────────┬───────┘
       │                                  │
       │ User/Dept queries                │ Repo queries
       ▼                                  ▼
┌──────────────────────┐      ┌──────────────────────┐
│  LDAP Manager        │      │  Gitea Service       │
│  - User Management   │◄─────│  - Repo filtering    │
│  - Dept Management   │ HTTP │  - Repo search       │
│  - GraphQL API       │      │  - GraphQL API       │
└──────────┬───────────┘      └──────────┬───────────┘
           │                             │
           │ LDAP Protocol               │ Gitea REST API
           ▼                             ▼
    ┌──────────┐                  ┌──────────┐
    │ OpenLDAP │                  │  Gitea   │
    └──────────┘                  └──────────┘
```

## Benefits of Migration

1. **Separation of Concerns**: Each service has a single responsibility
2. **Independent Scaling**: Scale Gitea Service independently based on repository query load
3. **Independent Deployment**: Deploy Gitea Service without affecting LDAP Manager
4. **Better Fault Isolation**: If Gitea Service fails, LDAP Manager continues working
5. **Technology Flexibility**: Can rewrite Gitea Service in different language if needed
6. **Team Ownership**: Different teams can own different services

## Migration Steps

### Step 1: Understand the Changes

**What Moved:**
- `backend/internal/gitea/client.go` → `gitea-service/internal/gitea/client.go`
- `backend/internal/gitea/service.go` → `gitea-service/internal/gitea/service.go` (modified)
- Gitea GraphQL queries → Moved from LDAP Manager schema to Gitea Service schema
- Gitea configuration → Separate ConfigMap and Secret

**What Changed:**
- Gitea Service now calls LDAP Manager via HTTP GraphQL to fetch user/department data
- Frontend now makes two types of calls:
  - LDAP Manager for user/department management
  - Gitea Service for repository queries

**What Stayed the Same:**
- LDAP Manager still manages users and departments
- JWT authentication mechanism (shared secret)
- LDAP attributes (`githubRepository` on users and departments)
- Repository access control logic

### Step 2: Deploy Gitea Service

Follow the deployment guide in `README.md`:

```bash
# 1. Build Docker image
cd gitea-service
docker build -t gitea-service:latest .

# 2. Create secret (IMPORTANT: Use same JWT_SECRET as LDAP Manager)
kubectl create secret generic gitea-service-secret \
  --from-literal=GITEA_TOKEN=your-gitea-admin-token \
  --from-literal=JWT_SECRET=$(kubectl get secret ldap-manager-secret -n dev-platform -o jsonpath='{.data.JWT_SECRET}' | base64 -d) \
  -n dev-platform

# 3. Deploy to Kubernetes
kubectl apply -f k8s/

# 4. Verify deployment
kubectl get pods -n dev-platform -l app=gitea-service
kubectl logs -f -l app=gitea-service -n dev-platform
```

### Step 3: Verify Inter-Service Communication

Test that Gitea Service can communicate with LDAP Manager:

```bash
# 1. Get JWT token from LDAP Manager
TOKEN=$(curl -s -X POST http://ldap-manager.localhost/graphql \
  -H "Content-Type: application/json" \
  -d '{"query":"mutation { login(uid: \"john.doe\", password: \"password123\") { token } }"}' | jq -r '.data.login.token')

# 2. Test Gitea Service health (includes LDAP Manager connectivity check)
curl http://gitea-service.localhost/graphql \
  -H "Content-Type: application/json" \
  -d '{"query":"query { health { status gitea ldapManager } }"}'

# 3. Test repository query with authentication
curl -X POST http://gitea-service.localhost/graphql \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query":"query { myRepositories { name fullName } }"}'
```

Expected output:
```json
{
  "data": {
    "myRepositories": [
      {
        "name": "api-gateway",
        "fullName": "devplatform/api-gateway"
      },
      {
        "name": "frontend",
        "fullName": "devplatform/frontend"
      }
    ]
  }
}
```

### Step 4: Update Frontend

Update your frontend GraphQL client to use two endpoints:

**Before (Single Endpoint):**
```typescript
// Single GraphQL client
const client = new ApolloClient({
  uri: 'http://ldap-manager.localhost/graphql',
  // ...
});

// All queries go to LDAP Manager
const { data } = useQuery(gql`
  query {
    users { uid mail }
    myGiteaRepositories { name }
  }
`);
```

**After (Two Endpoints):**
```typescript
// LDAP Manager client
const ldapClient = new ApolloClient({
  uri: 'http://ldap-manager.localhost/graphql',
  // ...
});

// Gitea Service client
const giteaClient = new ApolloClient({
  uri: 'http://gitea-service.localhost/graphql',
  // ...
});

// User queries go to LDAP Manager
const { data: userData } = useQuery(USERS_QUERY, { client: ldapClient });

// Repository queries go to Gitea Service
const { data: repoData } = useQuery(REPOSITORIES_QUERY, { client: giteaClient });
```

**Query Mapping:**

| Query/Mutation | Old Endpoint | New Endpoint |
|----------------|--------------|--------------|
| `login` | LDAP Manager | LDAP Manager |
| `me` | LDAP Manager | LDAP Manager |
| `users` | LDAP Manager | LDAP Manager |
| `createUser` | LDAP Manager | LDAP Manager |
| `updateUser` | LDAP Manager | LDAP Manager |
| `deleteUser` | LDAP Manager | LDAP Manager |
| `departments` | LDAP Manager | LDAP Manager |
| `createDepartment` | LDAP Manager | LDAP Manager |
| `assignRepoToUser` | LDAP Manager | LDAP Manager |
| `assignRepoToDepartment` | LDAP Manager | LDAP Manager |
| `myGiteaRepositories` → `myRepositories` | LDAP Manager | **Gitea Service** |
| `giteaRepository` → `repository` | LDAP Manager | **Gitea Service** |
| `searchGiteaRepositories` → `searchRepositories` | LDAP Manager | **Gitea Service** |
| `giteaRepositoryStats` → `repositoryStats` | LDAP Manager | **Gitea Service** |

**Updated Query Examples:**

```typescript
// Get user data (LDAP Manager)
const GET_USER = gql`
  query GetUser($uid: String!) {
    user(uid: $uid) {
      uid
      cn
      mail
      department
      repositories  # Still in LDAP
    }
  }
`;

// Get repositories (Gitea Service)
const GET_REPOSITORIES = gql`
  query GetRepositories {
    myRepositories {
      id
      name
      fullName
      description
      htmlUrl
    }
  }
`;

// Usage in component
function UserDashboard() {
  const { data: user } = useQuery(GET_USER, {
    variables: { uid: 'john.doe' },
    client: ldapClient
  });

  const { data: repos } = useQuery(GET_REPOSITORIES, {
    client: giteaClient
  });

  return (
    <div>
      <h1>Welcome {user?.user?.cn}</h1>
      <h2>Department: {user?.user?.department}</h2>
      <h2>Your Repositories:</h2>
      {repos?.myRepositories?.map(repo => (
        <div key={repo.id}>{repo.fullName}</div>
      ))}
    </div>
  );
}
```

### Step 5: Update Environment Configuration

If using environment variables for GraphQL endpoints:

**.env (Development):**
```bash
VITE_LDAP_MANAGER_URL=http://localhost:8080/graphql
VITE_GITEA_SERVICE_URL=http://localhost:8081/graphql
```

**.env.production:**
```bash
VITE_LDAP_MANAGER_URL=http://ldap-manager.localhost/graphql
VITE_GITEA_SERVICE_URL=http://gitea-service.localhost/graphql
```

### Step 6: Test End-to-End Flow

1. **Test Authentication:**
   ```bash
   # Login via LDAP Manager
   curl -X POST http://ldap-manager.localhost/graphql \
     -H "Content-Type: application/json" \
     -d '{"query":"mutation { login(uid: \"john.doe\", password: \"password123\") { token user { uid } } }"}'
   ```

2. **Test User Management (LDAP Manager):**
   ```bash
   curl -X POST http://ldap-manager.localhost/graphql \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer $TOKEN" \
     -d '{"query":"query { users { uid mail department repositories } }"}'
   ```

3. **Test Repository Access (Gitea Service):**
   ```bash
   curl -X POST http://gitea-service.localhost/graphql \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer $TOKEN" \
     -d '{"query":"query { myRepositories { name fullName } }"}'
   ```

4. **Test Repository Filtering:**
   - Assign a repository to a user in LDAP Manager
   - Query repositories from Gitea Service
   - Verify the newly assigned repo appears

### Step 7: Clean Up Old Code (Optional)

If migration is successful and stable, you can remove Gitea integration from LDAP Manager:

**Files to Remove:**
```bash
# In LDAP Manager backend
rm backend/internal/gitea/client.go
rm backend/internal/gitea/service.go
rm -rf backend/internal/gitea/
```

**Code to Remove from LDAP Manager:**

1. **backend/internal/config/config.go:**
   ```go
   // Remove these lines
   GiteaURL   string `envconfig:"GITEA_URL" required:"true"`
   GiteaToken string `envconfig:"GITEA_TOKEN" required:"true"`
   ```

2. **backend/internal/models/models.go:**
   ```go
   // Remove GiteaRepository struct and related types
   ```

3. **backend/internal/graphql/schema.go:**
   ```go
   // Remove Gitea-related GraphQL queries:
   // - myGiteaRepositories
   // - giteaRepository
   // - searchGiteaRepositories
   // - giteaRepositoryStats
   ```

4. **backend/cmd/server/main.go:**
   ```go
   // Remove Gitea client initialization:
   // giteaClient := gitea.NewClient(...)
   // giteaService := gitea.NewService(...)

   // Update GraphQL schema initialization to remove Gitea params
   ```

5. **backend/k8s/01-configmap.yaml:**
   ```yaml
   # Remove:
   # GITEA_URL: "..."
   # GITEA_TOKEN: "..."
   ```

6. **backend/GITEA_INTEGRATION.md:**
   ```bash
   rm backend/GITEA_INTEGRATION.md  # This doc is now obsolete
   ```

**After cleanup, rebuild and redeploy LDAP Manager:**
```bash
cd backend
docker build -t ldap-manager:latest .
kubectl rollout restart deployment/ldap-manager -n dev-platform
```

## Rollback Plan

If issues arise, you can rollback to monolithic architecture:

### Step 1: Scale Down Gitea Service
```bash
kubectl scale deployment gitea-service --replicas=0 -n dev-platform
```

### Step 2: Revert Frontend Changes
- Restore single GraphQL client configuration
- Point all queries back to LDAP Manager

### Step 3: Redeploy Old LDAP Manager
- Checkout previous version with Gitea integration
- Rebuild and redeploy

### Step 4: Delete Gitea Service (if needed)
```bash
kubectl delete -f gitea-service/k8s/
```

## Monitoring Migration

### Metrics to Watch

1. **LDAP Manager Metrics:**
   ```bash
   kubectl port-forward -n dev-platform svc/ldap-manager 9090:9090
   curl http://localhost:9090/metrics | grep ldap_operations_total
   ```

2. **Gitea Service Metrics:**
   ```bash
   kubectl port-forward -n dev-platform svc/gitea-service 9091:9091
   curl http://localhost:9091/metrics | grep gitea_operations_total
   ```

3. **Health Checks:**
   ```bash
   # LDAP Manager health
   curl http://ldap-manager.localhost/ready

   # Gitea Service health (includes dependency checks)
   curl http://gitea-service.localhost/ready
   ```

### Logs to Monitor

```bash
# LDAP Manager logs
kubectl logs -f -l app=ldap-manager -n dev-platform

# Gitea Service logs
kubectl logs -f -l app=gitea-service -n dev-platform

# Filter for errors
kubectl logs -l app=gitea-service -n dev-platform | grep ERROR
```

## Common Issues and Solutions

### Issue 1: JWT Token Not Working with Gitea Service

**Symptom:** Gitea Service returns "unauthorized" error

**Solution:**
1. Verify JWT secrets match:
   ```bash
   kubectl get secret ldap-manager-secret -n dev-platform -o jsonpath='{.data.JWT_SECRET}' | base64 -d
   kubectl get secret gitea-service-secret -n dev-platform -o jsonpath='{.data.JWT_SECRET}' | base64 -d
   ```
2. Ensure both outputs are identical
3. If different, update Gitea Service secret:
   ```bash
   kubectl delete secret gitea-service-secret -n dev-platform
   kubectl create secret generic gitea-service-secret \
     --from-literal=JWT_SECRET=$(kubectl get secret ldap-manager-secret -n dev-platform -o jsonpath='{.data.JWT_SECRET}' | base64 -d) \
     --from-literal=GITEA_TOKEN=your-gitea-token \
     -n dev-platform
   kubectl rollout restart deployment/gitea-service -n dev-platform
   ```

### Issue 2: Gitea Service Can't Reach LDAP Manager

**Symptom:** Readiness probe fails, health check shows `ldapManager: false`

**Solution:**
1. Check LDAP Manager is running:
   ```bash
   kubectl get pods -n dev-platform -l app=ldap-manager
   ```
2. Verify service DNS resolution from Gitea Service pod:
   ```bash
   kubectl exec -it deployment/gitea-service -n dev-platform -- nslookup ldap-manager.dev-platform.svc.cluster.local
   ```
3. Test HTTP connectivity:
   ```bash
   kubectl exec -it deployment/gitea-service -n dev-platform -- wget -O- http://ldap-manager.dev-platform.svc.cluster.local:8080/health
   ```

### Issue 3: No Repositories Returned

**Symptom:** User is authenticated but `myRepositories` returns empty array

**Solution:**
1. Check LDAP attributes are set correctly (see `README.md` troubleshooting)
2. Verify Gitea connectivity from Gitea Service
3. Check repository names format in LDAP (must be `owner/repo`)

### Issue 4: Frontend CORS Errors

**Symptom:** Browser console shows CORS errors when calling Gitea Service

**Solution:**
1. Update CORS configuration in ConfigMap:
   ```yaml
   # k8s/01-configmap.yaml
   data:
     CORS_ORIGINS: "http://your-frontend-url.localhost,http://another-url.localhost"
   ```
2. Apply changes:
   ```bash
   kubectl apply -f k8s/01-configmap.yaml
   kubectl rollout restart deployment/gitea-service -n dev-platform
   ```

## Validation Checklist

Use this checklist to verify successful migration:

- [ ] Gitea Service pods are running (2+ replicas)
- [ ] Health check returns status "ok"
- [ ] Readiness check shows both Gitea and LDAP Manager as healthy
- [ ] Can login via LDAP Manager and get JWT token
- [ ] JWT token works with Gitea Service
- [ ] User repositories are filtered correctly
- [ ] Department repositories are included
- [ ] Repository search works
- [ ] Repository stats are calculated correctly
- [ ] Frontend successfully calls both services
- [ ] Prometheus metrics are exposed for both services
- [ ] HPA is configured and working
- [ ] Pod disruption budget is in place
- [ ] Logs are structured and readable
- [ ] No error logs in either service
- [ ] Performance is acceptable (no degradation)

## Performance Considerations

### Expected Latency Changes

**Monolithic (Before):**
- Single request to LDAP Manager: ~50-100ms
- LDAP Manager fetches user data from LDAP: ~20ms
- LDAP Manager fetches repos from Gitea: ~30ms
- **Total: ~50-100ms**

**Microservices (After):**
- Frontend → Gitea Service: ~10ms
- Gitea Service → LDAP Manager: ~20ms
- LDAP Manager → LDAP: ~20ms (user data)
- LDAP Manager → LDAP: ~20ms (department data)
- Gitea Service → Gitea: ~30ms
- **Total: ~100-150ms**

**Impact:** Slight increase in latency (~50ms) due to additional network hop. This is acceptable for the benefits gained.

### Optimization Tips

1. **Implement Caching**: Cache LDAP responses in Gitea Service for 5-10 minutes
2. **Connection Pooling**: Ensure HTTP clients use keep-alive connections
3. **Parallel Requests**: Fetch user and department data in parallel
4. **Response Compression**: Enable gzip compression in both services

## Success Metrics

Track these metrics to measure migration success:

1. **Availability**: Target 99.9% uptime for both services
2. **Latency**: p95 < 200ms for repository queries
3. **Error Rate**: < 0.1% error rate
4. **Resource Usage**: CPU < 70%, Memory < 80% under normal load
5. **HPA Triggers**: Scaling events triggered appropriately under load

## Next Steps

After successful migration:

1. **Add Monitoring Dashboards**: Create Grafana dashboards for both services
2. **Set Up Alerts**: Configure alerts for high error rates, latency, downtime
3. **Document API**: Update API documentation for frontend developers
4. **Load Testing**: Perform load tests to verify scaling behavior
5. **Backup Strategy**: Implement backup for LDAP data (repositories assignments)
6. **CI/CD Pipeline**: Set up automated build and deployment
7. **Security Audit**: Perform security review of inter-service communication

## Additional Resources

- `README.md` - Gitea Service documentation
- `DEPLOYMENT_CHECKLIST.md` - Deployment verification steps (if exists)
- LDAP Manager documentation
- Kubernetes documentation

## Support

For issues during migration:
1. Check logs of both services
2. Verify configuration and secrets
3. Test inter-service connectivity
4. Consult troubleshooting sections in README.md
5. Contact DevOps team

---

**Migration completed?** Don't forget to update your runbooks, monitoring, and documentation!
