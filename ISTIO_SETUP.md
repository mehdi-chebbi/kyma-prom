# Istio Service Mesh Setup Guide

This guide covers the complete Istio service mesh integration for the KYMA Flow platform, including both the LDAP Manager backend and Gitea Service.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Services Configuration](#services-configuration)
- [Testing](#testing)
- [Monitoring](#monitoring)
- [Troubleshooting](#troubleshooting)
- [Security](#security)

## Overview

The KYMA Flow platform uses Istio service mesh to provide:

- **Mutual TLS (mTLS)**: Secure service-to-service communication
- **Traffic Management**: Intelligent routing, load balancing, retries
- **Observability**: Distributed tracing, metrics, logging
- **Security**: JWT authentication, authorization policies
- **Resilience**: Circuit breaking, timeouts, fault injection

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│              Istio Ingress Gateway                          │
│  HTTP/HTTPS (80/443)                                        │
└──────────────────┬──────────────────────────────────────────┘
                   │
         ┌─────────┴─────────┐
         │                   │
         ▼                   ▼
┌────────────────┐   ┌────────────────┐
│ LDAP Manager   │   │ Gitea Service  │
│ VirtualService │   │ VirtualService │
└────────┬───────┘   └────────┬───────┘
         │                    │
         ▼                    ▼
┌─────────────────────────────────────────┐
│        Service Mesh (mTLS)              │
│  ┌──────────────┐  ┌──────────────┐    │
│  │ LDAP Manager │←→│ Gitea Service│    │
│  │ [app|proxy]  │  │ [app|proxy]  │    │
│  └──────┬───────┘  └──────┬───────┘    │
│         │                  │            │
│         ▼                  ▼            │
│  ┌──────────────┐   ┌────────────┐     │
│  │  OpenLDAP    │   │ External   │     │
│  │   Server     │   │   Gitea    │     │
│  └──────────────┘   └────────────┘     │
└─────────────────────────────────────────┘
```

## Prerequisites

### 1. Kubernetes Cluster

Ensure you have a running Kubernetes cluster (e.g., Rancher):

```powershell
kubectl cluster-info
kubectl get nodes
```

### 2. Install Istio

**Option A: Using istioctl (Recommended)**

```powershell
# Download Istio
curl -L https://istio.io/downloadIstio | sh -

# Add istioctl to PATH
$env:Path += ";C:\path\to\istio-1.20.0\bin"

# Install Istio with default profile
istioctl install --set profile=default -y
```

**Option B: Using Helm**

```powershell
helm repo add istio https://istio-release.storage.googleapis.com/charts
helm repo update

# Install Istio base
helm install istio-base istio/base -n istio-system --create-namespace

# Install Istio discovery
helm install istiod istio/istiod -n istio-system --wait

# Install Istio ingress gateway
helm install istio-ingress istio/gateway -n istio-system
```

### 3. Verify Istio Installation

```powershell
# Check Istio components
kubectl get pods -n istio-system

# Verify installation
istioctl verify-install
```

## Installation

### Step 1: Enable Istio Injection

```powershell
# Label the namespace for automatic sidecar injection
kubectl create namespace dev-platform
kubectl label namespace dev-platform istio-injection=enabled

# Verify the label
kubectl get namespace dev-platform --show-labels
```

### Step 2: Deploy LDAP Manager with Istio

```powershell
# Navigate to backend directory
cd backend

# Run the deployment script
.\deploy-istio.ps1
```

This script will:
1. Check prerequisites (kubectl, Istio)
2. Create dev-platform namespace
3. Enable Istio sidecar injection
4. Deploy OpenLDAP
5. Initialize LDAP structure
6. Deploy LDAP Manager service
7. Deploy Istio configurations (Gateway, VirtualService, etc.)
8. Verify deployment
9. Test endpoints

### Step 3: Deploy Gitea Service with Istio

```powershell
# Navigate to gitea-service directory
cd ..\gitea-service

# Run the deployment script
.\deploy-istio.ps1
```

### Step 4: Verify Deployments

```powershell
# Check all pods have Istio sidecar (2/2 containers)
kubectl get pods -n dev-platform

# Verify Istio resources
kubectl get gateway,virtualservice,destinationrule,peerauthentication -n dev-platform
```

## Services Configuration

### LDAP Manager Service

**Endpoints:**
- **GraphQL API**: `/graphql` - Main GraphQL endpoint
- **Health Check**: `/health` - Liveness probe
- **Readiness**: `/ready` - Readiness probe
- **Metrics**: `/metrics` - Prometheus metrics (mesh-only)

**Istio Resources:**
- `ldap-manager-gateway`: HTTP/HTTPS ingress gateway
- `ldap-manager-vs`: VirtualService with routing rules
- `ldap-manager-dr`: DestinationRule with connection pooling and mTLS
- `ldap-manager-mtls`: PeerAuthentication enforcing STRICT mTLS
- `ldap-manager-jwt`: RequestAuthentication for JWT validation
- `ldap-manager-authz`: AuthorizationPolicy for access control

**Configuration Files:**
- `backend/k8s/04-istio-config.yaml` - Istio configuration
- `backend/ISTIO_DEPLOYMENT.md` - Detailed deployment guide
- `backend/deploy-istio.ps1` - PowerShell deployment script

### Gitea Service

**Endpoints:**
- **GraphQL API**: `/graphql` - Gitea GraphQL endpoint
- **Health Check**: `/health` - Liveness probe
- **Readiness**: `/ready` - Readiness probe
- **Metrics**: `/metrics` - Prometheus metrics (mesh-only)

**Istio Resources:**
- `gitea-service-gateway`: HTTP/HTTPS ingress gateway
- `gitea-service-vs`: VirtualService with routing rules
- `gitea-service-dr`: DestinationRule with connection pooling and mTLS
- `gitea-service-mtls`: PeerAuthentication enforcing STRICT mTLS
- `gitea-service-jwt`: RequestAuthentication for JWT validation
- `gitea-service-authz`: AuthorizationPolicy for access control

**Configuration Files:**
- `gitea-service/k8s/06-istio-config.yaml` - Istio configuration
- `gitea-service/ISTIO_DEPLOYMENT.md` - Detailed deployment guide
- `gitea-service/deploy-istio.ps1` - PowerShell deployment script

## Testing

### Get Ingress Gateway Information

```powershell
# Get Istio Ingress Gateway IP
$INGRESS_HOST = kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
$INGRESS_PORT = kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.spec.ports[?(@.name=="http2")].port}'

Write-Host "Ingress Host: $INGRESS_HOST"
Write-Host "Ingress Port: $INGRESS_PORT"
```

### Update Hosts File

Add to `C:\Windows\System32\drivers\etc\hosts` (requires admin):

```
<INGRESS_HOST> ldap-manager.localhost
<INGRESS_HOST> gitea-service.localhost
```

### Test LDAP Manager

**Health Check:**
```powershell
Invoke-WebRequest -Uri "http://$INGRESS_HOST:$INGRESS_PORT/health" `
  -Headers @{"Host"="ldap-manager.localhost"} -UseBasicParsing
```

**Login Mutation:**
```powershell
$body = '{"query":"mutation { login(uid: \"john.doe\", password: \"password123\") { token user { uid mail department } } }"}'
$response = Invoke-WebRequest -Uri "http://$INGRESS_HOST:$INGRESS_PORT/graphql" `
  -Method POST `
  -Headers @{"Host"="ldap-manager.localhost"; "Content-Type"="application/json"} `
  -Body $body -UseBasicParsing

$result = $response.Content | ConvertFrom-Json
$token = $result.data.login.token
Write-Host "JWT Token: $token"
```

**Authenticated Query:**
```powershell
$body = '{"query":"query { me { uid mail department repositories } }"}'
$response = Invoke-WebRequest -Uri "http://$INGRESS_HOST:$INGRESS_PORT/graphql" `
  -Method POST `
  -Headers @{"Host"="ldap-manager.localhost"; "Content-Type"="application/json"; "Authorization"="Bearer $token"} `
  -Body $body -UseBasicParsing

$response.Content
```

### Test Gitea Service

**Health Check:**
```powershell
Invoke-WebRequest -Uri "http://$INGRESS_HOST:$INGRESS_PORT/health" `
  -Headers @{"Host"="gitea-service.localhost"} -UseBasicParsing
```

### Test mTLS

```powershell
# Get LDAP Manager pod name
$POD_NAME = kubectl get pod -n dev-platform -l app=ldap-manager -o jsonpath='{.items[0].metadata.name}'

# Check mTLS status
istioctl authn tls-check $POD_NAME -n dev-platform

# Should show STRICT mode for all connections
```

## Monitoring

### Kiali Dashboard

Kiali provides visualization of the service mesh:

```powershell
# Install Kiali
kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.20/samples/addons/kiali.yaml

# Access Kiali dashboard
istioctl dashboard kiali
```

Navigate to:
- **Graph**: Visualize service communication
- **Applications**: View application health
- **Workloads**: Monitor workload status
- **Services**: Check service configuration
- **Istio Config**: Validate Istio resources

### Prometheus Metrics

```powershell
# Install Prometheus
kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.20/samples/addons/prometheus.yaml

# Access Prometheus
istioctl dashboard prometheus
```

**Key Metrics:**
- `istio_requests_total` - Total requests
- `istio_request_duration_milliseconds` - Request latency
- `istio_tcp_connections_opened_total` - TCP connections
- `envoy_cluster_upstream_cx_active` - Active connections

### Grafana Dashboards

```powershell
# Install Grafana
kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.20/samples/addons/grafana.yaml

# Access Grafana
istioctl dashboard grafana
```

**Built-in Dashboards:**
- Istio Mesh Dashboard
- Istio Service Dashboard
- Istio Workload Dashboard
- Istio Performance Dashboard

### Jaeger Tracing

```powershell
# Install Jaeger
kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.20/samples/addons/jaeger.yaml

# Access Jaeger UI
istioctl dashboard jaeger
```

### View Logs

```powershell
# LDAP Manager application logs
kubectl logs -f -l app=ldap-manager -n dev-platform -c ldap-manager

# LDAP Manager Istio proxy logs
kubectl logs -f -l app=ldap-manager -n dev-platform -c istio-proxy

# Gitea Service application logs
kubectl logs -f -l app=gitea-service -n dev-platform -c gitea-service

# Gitea Service Istio proxy logs
kubectl logs -f -l app=gitea-service -n dev-platform -c istio-proxy
```

## Troubleshooting

### Pods Not Getting Istio Sidecar

**Symptoms:** Pods show 1/1 containers instead of 2/2

**Solution:**
```powershell
# Verify namespace has istio-injection label
kubectl get namespace dev-platform --show-labels

# If missing, add the label
kubectl label namespace dev-platform istio-injection=enabled --overwrite

# Restart deployments
kubectl rollout restart deployment/ldap-manager -n dev-platform
kubectl rollout restart deployment/gitea-service -n dev-platform
```

### 503 Service Unavailable

**Symptoms:** Getting 503 errors when accessing services

**Solution:**
```powershell
# Check if pods are ready
kubectl get pods -n dev-platform

# Check service endpoints
kubectl get endpoints ldap-manager -n dev-platform
kubectl get endpoints gitea-service -n dev-platform

# Check DestinationRule
kubectl describe destinationrule ldap-manager-dr -n dev-platform

# Check Istio proxy status
kubectl logs -l app=ldap-manager -n dev-platform -c istio-proxy --tail=50
```

### mTLS Connection Errors

**Symptoms:** Connection refused or TLS errors

**Solution:**
```powershell
# Verify PeerAuthentication is configured
kubectl get peerauthentication -n dev-platform -o yaml

# Check certificate status
kubectl logs -l app=ldap-manager -n dev-platform -c istio-proxy | Select-String "certificate"

# Verify mTLS mode
istioctl authn tls-check <pod-name> -n dev-platform
```

### CORS Errors

**Symptoms:** Browser shows CORS policy errors

**Solution:**
```powershell
# Check VirtualService CORS configuration
kubectl get virtualservice ldap-manager-vs -n dev-platform -o yaml

# Verify allowOrigins includes your frontend URL
# Edit if needed:
kubectl edit virtualservice ldap-manager-vs -n dev-platform
```

### JWT Authentication Failures

**Symptoms:** GraphQL queries return authentication errors

**Solution:**
```powershell
# Verify RequestAuthentication configuration
kubectl get requestauthentication -n dev-platform -o yaml

# Check JWT token validity (decode JWT)
# PowerShell JWT decode:
$token = "<your-jwt-token>"
$parts = $token.Split('.')
$payload = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($parts[1]))
$payload | ConvertFrom-Json
```

### Gateway Not Receiving Traffic

**Symptoms:** Cannot access services through Ingress Gateway

**Solution:**
```powershell
# Check Ingress Gateway status
kubectl get svc istio-ingressgateway -n istio-system

# Check Gateway configuration
kubectl get gateway -n dev-platform

# Verify Gateway is bound to Ingress Gateway
kubectl describe gateway ldap-manager-gateway -n dev-platform

# Check Ingress Gateway logs
kubectl logs -l app=istio-ingressgateway -n istio-system
```

## Security

### Mutual TLS (mTLS)

All services are configured with **STRICT mTLS** mode:

```yaml
spec:
  mtls:
    mode: STRICT  # All connections must be encrypted
```

This ensures:
- All service-to-service communication is encrypted
- Man-in-the-middle attacks are prevented
- Service identity verification

### JWT Authentication

Services validate JWT tokens issued by ldap-manager:

```yaml
jwtRules:
- issuer: "ldap-manager"
  jwksUri: "http://ldap-manager.dev-platform.svc.cluster.local:8080/jwks"
```

### Authorization Policies

Fine-grained access control:

- **Public endpoints**: `/health`, `/ready` (no authentication required)
- **Login endpoint**: `/graphql` with login mutation (no authentication required)
- **Protected endpoints**: `/graphql` with other queries/mutations (JWT required)
- **Metrics endpoint**: `/metrics` (only from monitoring namespace)

### Network Policies

Consider adding Kubernetes NetworkPolicies for defense in depth:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: ldap-manager-netpol
  namespace: dev-platform
spec:
  podSelector:
    matchLabels:
      app: ldap-manager
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: istio-system
    - podSelector:
        matchLabels:
          app: gitea-service
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: openldap1
    ports:
    - protocol: TCP
      port: 389
```

## Best Practices

1. **Always use STRICT mTLS** for production environments
2. **Implement proper JWT validation** with short expiration times
3. **Use authorization policies** to enforce least privilege access
4. **Monitor service mesh metrics** for anomalies
5. **Enable distributed tracing** for debugging
6. **Set appropriate timeouts and retries** for resilience
7. **Use circuit breaking** to prevent cascading failures
8. **Regularly update Istio** to get security patches
9. **Implement rate limiting** to prevent abuse
10. **Test failover scenarios** regularly

## Performance Tuning

### Connection Pooling

Adjust connection pool settings based on load:

```yaml
connectionPool:
  tcp:
    maxConnections: 100  # Adjust based on load
  http:
    http1MaxPendingRequests: 50
    http2MaxRequests: 100
```

### Circuit Breaking

Configure outlier detection:

```yaml
outlierDetection:
  consecutiveErrors: 5      # Eject after 5 errors
  interval: 30s             # Check every 30s
  baseEjectionTime: 30s     # Eject for 30s
  maxEjectionPercent: 50    # Max 50% of hosts
```

### Resource Limits

Set appropriate limits for Istio proxy:

```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 2000m
    memory: 1024Mi
```

## Cleanup

To remove Istio and all configurations:

```powershell
# Delete Istio configurations
kubectl delete -f backend/k8s/04-istio-config.yaml
kubectl delete -f gitea-service/k8s/06-istio-config.yaml

# Remove Istio injection label
kubectl label namespace dev-platform istio-injection-

# Restart pods without sidecar
kubectl rollout restart deployment/ldap-manager -n dev-platform
kubectl rollout restart deployment/gitea-service -n dev-platform

# Uninstall Istio (optional)
istioctl uninstall --purge -y

# Delete Istio namespace
kubectl delete namespace istio-system
```

## References

- [Istio Documentation](https://istio.io/latest/docs/)
- [Istio Security Best Practices](https://istio.io/latest/docs/ops/best-practices/security/)
- [Istio Performance and Scalability](https://istio.io/latest/docs/ops/deployment/performance-and-scalability/)
- [Kiali Documentation](https://kiali.io/docs/)
- [LDAP Manager Istio Deployment](backend/ISTIO_DEPLOYMENT.md)
- [Gitea Service Istio Deployment](gitea-service/ISTIO_DEPLOYMENT.md)

## Support

For issues or questions:
1. Check the troubleshooting section above
2. Review Istio logs: `kubectl logs -l app=istio-ingressgateway -n istio-system`
3. Use `istioctl analyze` to validate configuration
4. Check service mesh visualization in Kiali

## Next Steps

1. ✅ Deploy both services with Istio
2. ✅ Verify mTLS is working
3. ✅ Test authentication flow
4. 🔲 Configure TLS certificates for HTTPS
5. 🔲 Set up monitoring dashboards
6. 🔲 Configure alerts for critical metrics
7. 🔲 Deploy frontend with Istio integration
8. 🔲 Implement rate limiting
9. 🔲 Set up GitOps for configuration management
10. 🔲 Perform load testing
