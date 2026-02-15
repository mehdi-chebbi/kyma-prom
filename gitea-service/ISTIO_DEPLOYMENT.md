# Istio Deployment Guide for Gitea Service

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
# Navigate to gitea-service directory
cd gitea-service

# Deploy ConfigMap and Secret
kubectl apply -f k8s/01-configmap.yaml
kubectl apply -f k8s/02-secret.yaml

# Deploy the service with Istio sidecar
kubectl apply -f k8s/03-deployment.yaml
kubectl apply -f k8s/04-service.yaml
```

### Step 2: Deploy Istio Configuration
```bash
# Deploy Istio Gateway, VirtualService, DestinationRule, etc.
kubectl apply -f k8s/06-istio-config.yaml
```

### Step 3: Verify Deployment
```bash
# Check if pods have Istio sidecar (should show 2/2 containers)
kubectl get pods -n dev-platform -l app=gitea-service

# Check Istio proxy status
kubectl get pods -n dev-platform -l app=gitea-service -o jsonpath='{.items[0].spec.containers[*].name}'
# Should show: gitea-service istio-proxy

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
curl -H "Host: gitea-service.localhost" http://$INGRESS_HOST:$INGRESS_PORT/health
```

### Test 2: GraphQL Endpoint
```bash
# Test GraphQL query
curl -X POST -H "Host: gitea-service.localhost" \
  -H "Content-Type: application/json" \
  -d '{"query":"{ __schema { types { name } } }"}' \
  http://$INGRESS_HOST:$INGRESS_PORT/graphql
```

### Test 3: mTLS Verification
```bash
# Check if mTLS is enforced
istioctl authn tls-check \
  $(kubectl get pod -n dev-platform -l app=gitea-service -o jsonpath='{.items[0].metadata.name}') \
  -n dev-platform

# Should show "STRICT" for mTLS mode
```

### Test 4: Internal Service Communication
```bash
# Port forward to test internal communication
kubectl port-forward -n dev-platform svc/gitea-service 8081:80

# In another terminal, test
curl http://localhost:8081/health
```

## Monitoring and Debugging

### View Istio Proxy Logs
```bash
# Get pod name
POD_NAME=$(kubectl get pod -n dev-platform -l app=gitea-service -o jsonpath='{.items[0].metadata.name}')

# View application logs
kubectl logs -n dev-platform $POD_NAME -c gitea-service

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
kubectl rollout restart deployment/gitea-service -n dev-platform
```

### Issue: 503 Service Unavailable

**Solution:**
```bash
# Check if DestinationRule is configured correctly
kubectl describe destinationrule gitea-service-dr -n dev-platform

# Check if service endpoints are available
kubectl get endpoints gitea-service -n dev-platform
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
kubectl get virtualservice gitea-service-vs -n dev-platform -o yaml
```

## Clean Up

To remove all Istio resources:
```bash
# Delete Istio configurations
kubectl delete -f k8s/06-istio-config.yaml

# Delete Istio injection label
kubectl label namespace dev-platform istio-injection-

# Restart pods without sidecar
kubectl rollout restart deployment/gitea-service -n dev-platform
```

## What's Configured

### ✅ Gateway
- HTTP on port 80
- HTTPS on port 443 (requires TLS secret)
- Hosts: gitea-service.localhost, gitea-service.local, gitea-api.local

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
- Authenticated access: /graphql
- Metrics access: monitoring namespace only

### ✅ ServiceEntry
- External Gitea instance configuration

## Next Steps

1. Test health and GraphQL endpoints
2. Verify mTLS is working
3. Check distributed tracing in Kiali
4. Monitor metrics in Grafana
5. Apply similar configuration to LDAP Manager service
