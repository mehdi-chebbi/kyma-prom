# Istio Deployment Guide for LDAP Manager Service

## Prerequisites

1. **Istio installed on your Rancher cluster**
   ```bash
   # Check if Istio is installed
   kubectl get pods -n istio-system

   # If not installed, install Istio
   istioctl install --set profile=default -y
   ```

2. **Enable Istio sidecar injection for dev-platform namespace**
   ```bash
   kubectl label namespace dev-platform istio-injection=enabled
   ```

3. **Verify Istio injection is enabled**
   ```bash
   kubectl get namespace dev-platform --show-labels
   ```

## Deployment Steps

### Step 1: Deploy Base Resources
```bash
# Navigate to backend directory
cd backend

# Deploy ConfigMap and Secret
kubectl apply -f k8s/01-openldap.yaml
kubectl apply -f k8s/02-ldap-init.yaml

# Wait for OpenLDAP to be ready
kubectl wait --for=condition=ready pod -l app=openldap1 -n dev-platform --timeout=180s

# Deploy the LDAP Manager service with Istio sidecar
kubectl apply -f k8s/03-ldap-manager.yaml
```

### Step 2: Deploy Istio Configuration
```bash
# Deploy Istio Gateway, VirtualService, DestinationRule, etc.
kubectl apply -f k8s/04-istio-config.yaml
```

### Step 3: Verify Deployment
```bash
# Check if pods have Istio sidecar (should show 2/2 containers)
kubectl get pods -n dev-platform -l app=ldap-manager

# Check Istio proxy status
kubectl get pods -n dev-platform -l app=ldap-manager -o jsonpath='{.items[0].spec.containers[*].name}'
# Should show: ldap-manager istio-proxy

# Verify Gateway
kubectl get gateway -n dev-platform

# Verify VirtualService
kubectl get virtualservice -n dev-platform

# Verify DestinationRule
kubectl get destinationrule -n dev-platform

# Check PeerAuthentication (mTLS)
kubectl get peerauthentication -n dev-platform
```

## Testing

### Test 1: Health Check
```bash
# Get Istio Ingress Gateway external IP
export INGRESS_HOST=$(kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
export INGRESS_PORT=$(kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.spec.ports[?(@.name=="http2")].port}')

# Test health endpoint
curl -H "Host: ldap-manager.localhost" http://$INGRESS_HOST:$INGRESS_PORT/health
```

### Test 2: GraphQL Login
```bash
# Test login mutation
curl -X POST -H "Host: ldap-manager.localhost" \
  -H "Content-Type: application/json" \
  -d '{"query":"mutation { login(uid: \"john.doe\", password: \"password123\") { token user { uid mail department } } }"}' \
  http://$INGRESS_HOST:$INGRESS_PORT/graphql
```

### Test 3: Authenticated GraphQL Query
```bash
# Get JWT token from login response above
export JWT_TOKEN="<token_from_login>"

# Test authenticated query
curl -X POST -H "Host: ldap-manager.localhost" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -d '{"query":"query { me { uid mail department repositories } }"}' \
  http://$INGRESS_HOST:$INGRESS_PORT/graphql
```

### Test 4: mTLS Verification
```bash
# Check if mTLS is enforced
istioctl authn tls-check \
  $(kubectl get pod -n dev-platform -l app=ldap-manager -o jsonpath='{.items[0].metadata.name}') \
  -n dev-platform

# Should show "STRICT" for mTLS mode
```

### Test 5: Internal Service Communication
```bash
# Port forward to test internal communication
kubectl port-forward -n dev-platform svc/ldap-manager 8081:30008

# In another terminal, test
curl http://localhost:8081/health
```

## Monitoring and Debugging

### View Istio Proxy Logs
```bash
# Get pod name
POD_NAME=$(kubectl get pod -n dev-platform -l app=ldap-manager -o jsonpath='{.items[0].metadata.name}')

# View application logs
kubectl logs -n dev-platform $POD_NAME -c ldap-manager

# View Istio proxy logs
kubectl logs -n dev-platform $POD_NAME -c istio-proxy
```

### Check Istio Configuration
```bash
# Check proxy configuration
istioctl proxy-config routes $POD_NAME -n dev-platform

# Check listeners
istioctl proxy-config listeners $POD_NAME -n dev-platform

# Check clusters
istioctl proxy-config clusters $POD_NAME -n dev-platform
```

### Verify Traffic Flow
```bash
# Enable debug logging on Envoy proxy
kubectl exec -it $POD_NAME -c istio-proxy -n dev-platform -- curl -X POST http://localhost:15000/logging?level=debug

# Check metrics
kubectl exec -it $POD_NAME -c istio-proxy -n dev-platform -- curl http://localhost:15090/stats/prometheus
```

## Kiali Dashboard (Optional)

If you have Kiali installed, you can visualize your service mesh:

```bash
# Install Kiali (if not installed)
kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.20/samples/addons/kiali.yaml

# Access Kiali dashboard
istioctl dashboard kiali
```

## Troubleshooting

### Issue: Pods not getting Istio sidecar

**Solution:**
```bash
# Ensure namespace has istio-injection label
kubectl label namespace dev-platform istio-injection=enabled --overwrite

# Restart pods to inject sidecar
kubectl rollout restart deployment/ldap-manager -n dev-platform
```

### Issue: 503 Service Unavailable

**Solution:**
```bash
# Check if DestinationRule is configured correctly
kubectl describe destinationrule ldap-manager-dr -n dev-platform

# Check if service endpoints are available
kubectl get endpoints ldap-manager -n dev-platform
```

### Issue: mTLS connection errors

**Solution:**
```bash
# Check PeerAuthentication policy
kubectl get peerauthentication -n dev-platform -o yaml

# Verify certificate rotation
kubectl logs -n dev-platform $POD_NAME -c istio-proxy | grep -i certificate
```

### Issue: CORS errors

**Solution:**
The VirtualService has CORS policy configured. Verify it matches your frontend origin:
```bash
kubectl get virtualservice ldap-manager-vs -n dev-platform -o yaml
```

### Issue: JWT authentication failing

**Solution:**
```bash
# Check RequestAuthentication configuration
kubectl get requestauthentication -n dev-platform -o yaml

# Verify JWT token is valid
echo $JWT_TOKEN | cut -d'.' -f2 | base64 -d | jq
```

### Issue: LDAP connection failures

**Solution:**
```bash
# Check OpenLDAP is running
kubectl get pods -n dev-platform -l app=openldap1

# Test LDAP connectivity from ldap-manager pod
kubectl exec -it $POD_NAME -c ldap-manager -n dev-platform -- nc -zv openldap1.dev-platform.svc.cluster.local 389

# Check ServiceEntry for OpenLDAP
kubectl get serviceentry external-openldap -n dev-platform -o yaml
```

## Clean Up

To remove all Istio resources:
```bash
# Delete Istio configurations
kubectl delete -f k8s/04-istio-config.yaml

# Delete Istio injection label
kubectl label namespace dev-platform istio-injection-

# Restart pods without sidecar
kubectl rollout restart deployment/ldap-manager -n dev-platform
```

## What's Configured

### ✅ Gateway
- HTTP on port 80
- HTTPS on port 443 (requires TLS secret)
- Hosts: ldap-manager.localhost, ldap-manager.local, ldap-api.local

### ✅ VirtualService
- GraphQL endpoint with CORS
- Health and readiness checks
- Metrics endpoint (mesh-only)
- 30s timeout, 3 retries

### ✅ DestinationRule
- Connection pooling (100 max connections)
- Load balancing: LEAST_REQUEST
- Circuit breaking (outlier detection)
- **Strict mTLS enforced**

### ✅ PeerAuthentication
- **STRICT mTLS mode** - all traffic must be encrypted

### ✅ RequestAuthentication
- JWT validation from ldap-manager issuer

### ✅ AuthorizationPolicy
- Public access: /health, /ready
- Authenticated access: /graphql (login mutation is public)
- Metrics access: monitoring namespace only

### ✅ ServiceEntry
- OpenLDAP internal service configuration

### ✅ Internal VirtualService
- Communication with gitea-service via mesh

## Architecture

```
┌─────────────────────────────────────────────────┐
│         Istio Ingress Gateway                   │
│  HTTP/HTTPS (80/443)                            │
└──────────────────┬──────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────┐
│      VirtualService (ldap-manager-vs)           │
│  Routes: /graphql, /health, /ready, /metrics    │
└──────────────────┬──────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────┐
│         LDAP Manager Service                    │
│  Pod 1 [ldap-manager | istio-proxy]             │
│  Pod 2 [ldap-manager | istio-proxy]             │
│  - GraphQL API                                  │
│  - JWT Auth                                     │
│  - mTLS enforced                                │
└──────────────────┬──────────────────────────────┘
                   │ LDAP (389)
                   ▼
┌─────────────────────────────────────────────────┐
│         OpenLDAP Service                        │
│  dc=devplatform,dc=local                        │
└─────────────────────────────────────────────────┘
```

## Security Features

1. **Strict mTLS**: All service-to-service communication is encrypted
2. **JWT Authentication**: GraphQL endpoints require valid JWT tokens (except login)
3. **Authorization Policies**: Fine-grained access control
4. **CORS Protection**: Configured for specific frontend origins
5. **Circuit Breaking**: Automatic failure isolation
6. **Rate Limiting**: Connection pool limits prevent overload

## Performance Features

1. **Load Balancing**: LEAST_REQUEST algorithm
2. **Connection Pooling**: Reuse connections efficiently
3. **Retry Logic**: Automatic retry on transient failures
4. **Timeouts**: Prevent hanging requests
5. **Horizontal Pod Autoscaling**: Scale based on load (2-5 replicas)

## Next Steps

1. Test health and GraphQL endpoints
2. Verify mTLS is working between services
3. Check distributed tracing in Kiali
4. Monitor metrics in Grafana
5. Configure TLS certificates for HTTPS
6. Set up monitoring alerts
7. Deploy frontend with Istio integration
