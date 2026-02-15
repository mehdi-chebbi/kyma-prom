# Quick Start Guide - Gitea Service

Get the Gitea Service up and running in 5 minutes.

## Prerequisites

- Kubernetes cluster running
- LDAP Manager service deployed and accessible
- Gitea server deployed and accessible
- `kubectl` configured to access your cluster
- Docker installed (for building the image)

## Quick Deployment

### Step 1: Clone and Navigate

```bash
cd gitea-service
```

### Step 2: Build Docker Image

```bash
docker build -t gitea-service:latest .
```

### Step 3: Configure Secrets

**Get the JWT secret from LDAP Manager** (CRITICAL - must match):

```bash
export JWT_SECRET=$(kubectl get secret ldap-manager-secret -n dev-platform -o jsonpath='{.data.JWT_SECRET}' | base64 -d)
echo "JWT Secret: $JWT_SECRET"
```

**Get or create a Gitea admin token**:

1. Login to Gitea UI
2. Go to Settings → Applications
3. Generate New Token with name "gitea-service"
4. Copy the token

**Create the secret:**

```bash
kubectl create secret generic gitea-service-secret \
  --from-literal=GITEA_TOKEN=YOUR_GITEA_TOKEN_HERE \
  --from-literal=JWT_SECRET="$JWT_SECRET" \
  -n dev-platform
```

### Step 4: Review Configuration

Edit `k8s/01-configmap.yaml` if needed:

```yaml
data:
  GITEA_URL: "http://gitea.dev-platform.svc.cluster.local:3000"  # Update if different
  LDAP_MANAGER_URL: "http://ldap-manager.dev-platform.svc.cluster.local:8080"  # Update if different
```

### Step 5: Deploy to Kubernetes

```bash
kubectl apply -f k8s/
```

### Step 6: Verify Deployment

```bash
# Check pods are running
kubectl get pods -n dev-platform -l app=gitea-service

# Expected output:
# NAME                             READY   STATUS    RESTARTS   AGE
# gitea-service-xxxxxxxxxx-xxxxx   1/1     Running   0          30s
# gitea-service-xxxxxxxxxx-xxxxx   1/1     Running   0          30s

# Check logs
kubectl logs -f -l app=gitea-service -n dev-platform --tail=20

# Should see:
# {"level":"info","message":"Starting Gitea Microservice","timestamp":"..."}
# {"level":"info","message":"Gitea connection successful","timestamp":"..."}
# {"level":"info","message":"LDAP Manager connection successful","timestamp":"..."}
# {"level":"info","message":"Starting HTTP server","port":8081,"timestamp":"..."}
```

### Step 7: Test the Service

**Health check (no auth required):**

```bash
kubectl port-forward -n dev-platform svc/gitea-service 8081:80

curl http://localhost:8081/health

# Expected output:
# {"status":"ok"}
```

**Readiness check (tests dependencies):**

```bash
curl http://localhost:8081/ready

# Expected output:
# {"gitea":true,"ldapManager":true,"status":"ready"}
```

**GraphQL query (requires authentication):**

```bash
# 1. Get JWT token from LDAP Manager
TOKEN=$(curl -s -X POST http://ldap-manager.localhost/graphql \
  -H "Content-Type: application/json" \
  -d '{"query":"mutation { login(uid: \"john.doe\", password: \"password123\") { token } }"}' | \
  jq -r '.data.login.token')

echo "Token: $TOKEN"

# 2. Query repositories
curl -X POST http://localhost:8081/graphql \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query":"query { myRepositories { name fullName description } }"}'

# Expected output:
# {
#   "data": {
#     "myRepositories": [
#       {
#         "name": "api-gateway",
#         "fullName": "devplatform/api-gateway",
#         "description": "API Gateway for microservices"
#       }
#     ]
#   }
# }
```

## Common Issues

### Issue: Pods CrashLoopBackOff

```bash
# Check logs for error
kubectl logs -l app=gitea-service -n dev-platform

# Common causes:
# 1. Secret not found → Create secret (Step 3)
# 2. Gitea not accessible → Check GITEA_URL in ConfigMap
# 3. LDAP Manager not accessible → Check LDAP_MANAGER_URL in ConfigMap
```

### Issue: Readiness Probe Failing

```bash
# Check which dependency is failing
curl http://localhost:8081/ready

# If gitea: false
kubectl port-forward -n dev-platform svc/gitea 3000:3000
curl http://localhost:3000/api/v1/repos/search

# If ldapManager: false
kubectl get pods -n dev-platform -l app=ldap-manager
```

### Issue: Unauthorized Errors

```bash
# Verify JWT secrets match
echo "LDAP Manager JWT:"
kubectl get secret ldap-manager-secret -n dev-platform -o jsonpath='{.data.JWT_SECRET}' | base64 -d
echo ""

echo "Gitea Service JWT:"
kubectl get secret gitea-service-secret -n dev-platform -o jsonpath='{.data.JWT_SECRET}' | base64 -d
echo ""

# If they don't match, recreate Gitea Service secret with correct JWT_SECRET
```

### Issue: No Repositories Returned

```bash
# Check LDAP attributes for test user
kubectl exec -it -n dev-platform deployment/ldap-manager -- sh

# Inside the pod:
ldapsearch -x -H ldap://openldap:389 \
  -b "dc=devplatform,dc=local" \
  -D "cn=admin,dc=devplatform,dc=local" \
  -w admin \
  "(uid=john.doe)" \
  githubRepository

# Should see:
# githubRepository: devplatform/api-gateway
# githubRepository: devplatform/frontend
```

## Quick Test Commands

### Test Complete Flow

```bash
# 1. Login
TOKEN=$(curl -s -X POST http://ldap-manager.localhost/graphql \
  -H "Content-Type: application/json" \
  -d '{"query":"mutation { login(uid: \"john.doe\", password: \"password123\") { token user { uid } } }"}' | \
  jq -r '.data.login.token')

# 2. Get my repositories
curl -s -X POST http://gitea-service.localhost/graphql \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query":"query { myRepositories { name } }"}' | jq

# 3. Search repositories
curl -s -X POST http://gitea-service.localhost/graphql \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query":"query { searchRepositories(query: \"api\") { name } }"}' | jq

# 4. Get statistics
curl -s -X POST http://gitea-service.localhost/graphql \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query":"query { repositoryStats { totalRepositories privateRepositories publicRepositories } }"}' | jq
```

### Test Specific Repository Access

```bash
curl -s -X POST http://gitea-service.localhost/graphql \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query":"query { repository(owner: \"devplatform\", name: \"api-gateway\") { name fullName description htmlUrl } }"}' | jq
```

### Monitor Logs

```bash
# Follow logs
kubectl logs -f -l app=gitea-service -n dev-platform

# Filter for errors
kubectl logs -l app=gitea-service -n dev-platform | grep -i error

# Filter for authentication logs
kubectl logs -l app=gitea-service -n dev-platform | grep -i auth
```

### Check Metrics

```bash
kubectl port-forward -n dev-platform svc/gitea-service 9091:9091
curl http://localhost:9091/metrics | grep gitea_operations_total
```

## Next Steps

1. **Update Frontend**: Point repository queries to Gitea Service (see `MIGRATION.md`)
2. **Set Up Monitoring**: Add Prometheus scraping and Grafana dashboards
3. **Configure Ingress**: Update ingress hostname for your domain
4. **Enable TLS**: Add TLS certificates to Ingress
5. **Review Logs**: Monitor logs for any errors or warnings

## Configuration Reference

### Environment Variables (ConfigMap)

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GITEA_URL` | Yes | - | Gitea server URL |
| `LDAP_MANAGER_URL` | Yes | - | LDAP Manager service URL |
| `PORT` | No | 8081 | HTTP server port |
| `METRICS_PORT` | No | 9091 | Metrics port |
| `ENVIRONMENT` | No | production | Runtime environment |
| `LOG_LEVEL` | No | info | Log level (debug, info, warn, error) |
| `HTTP_CLIENT_TIMEOUT` | No | 30s | HTTP client timeout |
| `JWT_EXPIRATION` | No | 24h | JWT expiration time |
| `CORS_ORIGINS` | No | * | Allowed CORS origins |
| `SHUTDOWN_TIMEOUT` | No | 30 | Graceful shutdown timeout (seconds) |

### Secrets (Secret)

| Secret | Required | Description |
|--------|----------|-------------|
| `GITEA_TOKEN` | Yes | Gitea admin token from Gitea UI |
| `JWT_SECRET` | Yes | **Must match LDAP Manager's JWT_SECRET** |

## Useful Commands

### Deployment Management

```bash
# Scale replicas
kubectl scale deployment gitea-service --replicas=3 -n dev-platform

# Restart pods
kubectl rollout restart deployment/gitea-service -n dev-platform

# Check rollout status
kubectl rollout status deployment/gitea-service -n dev-platform

# View deployment history
kubectl rollout history deployment/gitea-service -n dev-platform
```

### Debugging

```bash
# Get pod details
kubectl describe pod -l app=gitea-service -n dev-platform

# Exec into pod
kubectl exec -it deployment/gitea-service -n dev-platform -- sh

# Test Gitea connectivity from pod
kubectl exec -it deployment/gitea-service -n dev-platform -- wget -O- http://gitea.dev-platform.svc.cluster.local:3000/api/v1/repos/search

# Test LDAP Manager connectivity from pod
kubectl exec -it deployment/gitea-service -n dev-platform -- wget -O- http://ldap-manager.dev-platform.svc.cluster.local:8080/health
```

### Resource Management

```bash
# View resource usage
kubectl top pod -l app=gitea-service -n dev-platform

# View HPA status
kubectl get hpa gitea-service -n dev-platform

# View events
kubectl get events -n dev-platform --sort-by='.lastTimestamp' | grep gitea-service
```

## Clean Up

To remove the Gitea Service:

```bash
# Delete all resources
kubectl delete -f k8s/

# Or delete individually
kubectl delete deployment gitea-service -n dev-platform
kubectl delete service gitea-service -n dev-platform
kubectl delete ingressroute gitea-service -n dev-platform
kubectl delete hpa gitea-service -n dev-platform
kubectl delete pdb gitea-service -n dev-platform
kubectl delete configmap gitea-service-config -n dev-platform
kubectl delete secret gitea-service-secret -n dev-platform
kubectl delete serviceaccount gitea-service -n dev-platform
```

## Documentation

For more detailed information:
- `README.md` - Complete documentation
- `MIGRATION.md` - Migration guide from monolithic backend
- `k8s/` - Kubernetes manifests with inline comments

## Support

If you encounter issues:
1. Check logs: `kubectl logs -f -l app=gitea-service -n dev-platform`
2. Check health: `curl http://gitea-service.localhost/ready`
3. Verify secrets: Ensure JWT_SECRET matches LDAP Manager
4. Test connectivity: Verify Gitea and LDAP Manager are accessible
5. Review configuration: Check ConfigMap values

---

**Deployment complete?** You should now be able to query repositories through the Gitea Service!
