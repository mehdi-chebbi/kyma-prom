# PowerShell script to deploy Gitea Service with Istio

Write-Host "🚀 Deploying Gitea Service with Istio..." -ForegroundColor Cyan
Write-Host ""

# Check if kubectl is available
Write-Host "📋 Checking prerequisites..." -ForegroundColor Yellow
try {
    kubectl version --client | Out-Null
} catch {
    Write-Host "❌ kubectl is not installed or not in PATH" -ForegroundColor Red
    exit 1
}

# Check if Istio is installed
try {
    kubectl get namespace istio-system 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) {
        Write-Host "❌ Istio is not installed. Please install Istio first." -ForegroundColor Red
        Write-Host "   Run: istioctl install --set profile=default -y" -ForegroundColor Yellow
        exit 1
    }
    Write-Host "✅ Istio is installed" -ForegroundColor Green
} catch {
    Write-Host "❌ Failed to check Istio installation" -ForegroundColor Red
    exit 1
}

# Check if namespace exists, create if not
Write-Host ""
try {
    kubectl get namespace dev-platform 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) {
        Write-Host "📦 Creating dev-platform namespace..." -ForegroundColor Yellow
        kubectl create namespace dev-platform
    }
} catch {
    Write-Host "📦 Creating dev-platform namespace..." -ForegroundColor Yellow
    kubectl create namespace dev-platform
}

# Enable Istio injection
Write-Host "🔧 Enabling Istio sidecar injection for dev-platform namespace..." -ForegroundColor Yellow
kubectl label namespace dev-platform istio-injection=enabled --overwrite
Write-Host "✅ Istio injection enabled" -ForegroundColor Green

# Deploy ConfigMap and Secret
Write-Host ""
Write-Host "📝 Deploying ConfigMap and Secret..." -ForegroundColor Yellow
kubectl apply -f k8s/01-configmap.yaml
kubectl apply -f k8s/02-secret.yaml
Write-Host "✅ ConfigMap and Secret deployed" -ForegroundColor Green

# Deploy application
Write-Host ""
Write-Host "🚢 Deploying Gitea Service..." -ForegroundColor Yellow
kubectl apply -f k8s/03-deployment.yaml
kubectl apply -f k8s/04-service.yaml
Write-Host "✅ Gitea Service deployed" -ForegroundColor Green

# Wait for pods to be ready
Write-Host ""
Write-Host "⏳ Waiting for pods to be ready (with Istio sidecar)..." -ForegroundColor Yellow
kubectl wait --for=condition=ready pod -l app=gitea-service -n dev-platform --timeout=120s
if ($LASTEXITCODE -ne 0) {
    Write-Host "⚠️  Pods not ready within timeout, continuing anyway..." -ForegroundColor Yellow
}

# Deploy Istio configuration
Write-Host ""
Write-Host "🌐 Deploying Istio Gateway, VirtualService, and Policies..." -ForegroundColor Yellow
kubectl apply -f k8s/06-istio-config.yaml
Write-Host "✅ Istio configuration deployed" -ForegroundColor Green

# Verify deployment
Write-Host ""
Write-Host "🔍 Verifying deployment..." -ForegroundColor Cyan
Write-Host ""

# Check pods
Write-Host "Pods:" -ForegroundColor Yellow
kubectl get pods -n dev-platform -l app=gitea-service

Write-Host ""
Write-Host "Gateway:" -ForegroundColor Yellow
kubectl get gateway -n dev-platform

Write-Host ""
Write-Host "VirtualService:" -ForegroundColor Yellow
kubectl get virtualservice -n dev-platform

Write-Host ""
Write-Host "DestinationRule:" -ForegroundColor Yellow
kubectl get destinationrule -n dev-platform

Write-Host ""
Write-Host "PeerAuthentication:" -ForegroundColor Yellow
kubectl get peerauthentication -n dev-platform

# Get Istio Ingress Gateway info
Write-Host ""
Write-Host "🌍 Istio Ingress Gateway Info:" -ForegroundColor Cyan

try {
    $INGRESS_HOST = kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>$null
    if ([string]::IsNullOrEmpty($INGRESS_HOST)) {
        $INGRESS_HOST = "pending"
    }
} catch {
    $INGRESS_HOST = "pending"
}

$INGRESS_PORT = kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.spec.ports[?(@.name=="http2")].port}'

Write-Host "   Ingress Host: $INGRESS_HOST" -ForegroundColor White
Write-Host "   Ingress Port: $INGRESS_PORT" -ForegroundColor White

if ($INGRESS_HOST -ne "pending") {
    Write-Host ""
    Write-Host "🧪 Testing endpoints..." -ForegroundColor Cyan
    Write-Host ""

    # Test health endpoint
    Write-Host -NoNewline "Testing /health... "
    try {
        $response = Invoke-WebRequest -Uri "http://$INGRESS_HOST:$INGRESS_PORT/health" -Headers @{"Host"="gitea-service.localhost"} -UseBasicParsing -TimeoutSec 5 -ErrorAction Stop
        if ($response.StatusCode -eq 200) {
            Write-Host "✅ OK" -ForegroundColor Green
        } else {
            Write-Host "⚠️  Not ready yet (Status: $($response.StatusCode))" -ForegroundColor Yellow
        }
    } catch {
        Write-Host "⚠️  Not ready yet" -ForegroundColor Yellow
    }

    Write-Host ""
    Write-Host "📌 Access URLs:" -ForegroundColor Cyan
    Write-Host "   Health:  http://$INGRESS_HOST:$INGRESS_PORT/health (Host: gitea-service.localhost)" -ForegroundColor White
    Write-Host "   GraphQL: http://$INGRESS_HOST:$INGRESS_PORT/graphql (Host: gitea-service.localhost)" -ForegroundColor White
    Write-Host ""
    Write-Host "💡 Add to C:\Windows\System32\drivers\etc\hosts:" -ForegroundColor Yellow
    Write-Host "   $INGRESS_HOST gitea-service.localhost" -ForegroundColor White
} else {
    Write-Host ""
    Write-Host "⚠️  Ingress Gateway LoadBalancer IP is pending..." -ForegroundColor Yellow
    Write-Host "   Run 'kubectl get svc -n istio-system' to check status" -ForegroundColor White
}

Write-Host ""
Write-Host "✅ Deployment complete!" -ForegroundColor Green
Write-Host ""
Write-Host "📚 Next steps:" -ForegroundColor Cyan
Write-Host "   1. Check logs: kubectl logs -f -l app=gitea-service -n dev-platform -c gitea-service" -ForegroundColor White
Write-Host "   2. Check proxy: kubectl logs -f -l app=gitea-service -n dev-platform -c istio-proxy" -ForegroundColor White
Write-Host "   3. Verify mTLS: istioctl authn tls-check POD_NAME -n dev-platform" -ForegroundColor White
Write-Host "   4. View in Kiali: istioctl dashboard kiali" -ForegroundColor White
Write-Host ""
