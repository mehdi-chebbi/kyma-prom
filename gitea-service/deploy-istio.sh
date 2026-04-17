#!/bin/bash

set -e

echo "🚀 Deploying Gitea Service with Istio..."
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Check if Istio is installed
echo "📋 Checking prerequisites..."
if ! kubectl get namespace istio-system &> /dev/null; then
    echo -e "${RED}❌ Istio is not installed. Please install Istio first.${NC}"
    echo "   Run: istioctl install --set profile=default -y"
    exit 1
fi
echo -e "${GREEN}✅ Istio is installed${NC}"

# Check if namespace exists
if ! kubectl get namespace dev-platform &> /dev/null; then
    echo "📦 Creating dev-platform namespace..."
    kubectl create namespace dev-platform
fi

# Enable Istio injection
echo "🔧 Enabling Istio sidecar injection for dev-platform namespace..."
kubectl label namespace dev-platform istio-injection=enabled --overwrite
echo -e "${GREEN}✅ Istio injection enabled${NC}"

# Deploy ConfigMap and Secret
echo ""
echo "📝 Deploying ConfigMap and Secret..."
kubectl apply -f k8s/01-configmap.yaml
kubectl apply -f k8s/02-secret.yaml
echo -e "${GREEN}✅ ConfigMap and Secret deployed${NC}"

# Deploy application
echo ""
echo "🚢 Deploying Gitea Service..."
kubectl apply -f k8s/03-deployment.yaml
kubectl apply -f k8s/04-service.yaml
echo -e "${GREEN}✅ Gitea Service deployed${NC}"

# Wait for pods to be ready
echo ""
echo "⏳ Waiting for pods to be ready (with Istio sidecar)..."
kubectl wait --for=condition=ready pod -l app=gitea-service -n dev-platform --timeout=120s || true

# Deploy Istio configuration
echo ""
echo "🌐 Deploying Istio Gateway, VirtualService, and Policies..."
kubectl apply -f k8s/06-istio-config.yaml
echo -e "${GREEN}✅ Istio configuration deployed${NC}"

# Verify deployment
echo ""
echo "🔍 Verifying deployment..."
echo ""

# Check pods
echo "Pods:"
kubectl get pods -n dev-platform -l app=gitea-service

echo ""
echo "Gateway:"
kubectl get gateway -n dev-platform

echo ""
echo "VirtualService:"
kubectl get virtualservice -n dev-platform

echo ""
echo "DestinationRule:"
kubectl get destinationrule -n dev-platform

echo ""
echo "PeerAuthentication:"
kubectl get peerauthentication -n dev-platform

# Get Istio Ingress Gateway info
echo ""
echo "🌍 Istio Ingress Gateway Info:"
INGRESS_HOST=$(kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "pending")
INGRESS_PORT=$(kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.spec.ports[?(@.name=="http2")].port}')

echo "   Ingress Host: $INGRESS_HOST"
echo "   Ingress Port: $INGRESS_PORT"

if [ "$INGRESS_HOST" != "pending" ]; then
    echo ""
    echo "🧪 Testing endpoints..."
    echo ""

    # Test health endpoint
    echo -n "Testing /health... "
    if curl -sf -H "Host: gitea-service.localhost" "http://$INGRESS_HOST:$INGRESS_PORT/health" > /dev/null; then
        echo -e "${GREEN}✅ OK${NC}"
    else
        echo -e "${YELLOW}⚠️  Not ready yet${NC}"
    fi

    echo ""
    echo "📌 Access URLs:"
    echo "   Health:  http://$INGRESS_HOST:$INGRESS_PORT/health (Host: gitea-service.localhost)"
    echo "   GraphQL: http://$INGRESS_HOST:$INGRESS_PORT/graphql (Host: gitea-service.localhost)"
    echo ""
    echo "💡 Add to /etc/hosts:"
    echo "   $INGRESS_HOST gitea-service.localhost"
else
    echo -e "${YELLOW}⚠️  Ingress Gateway LoadBalancer IP is pending...${NC}"
    echo "   Run 'kubectl get svc -n istio-system' to check status"
fi

echo ""
echo -e "${GREEN}✅ Deployment complete!${NC}"
echo ""
echo "📚 Next steps:"
echo "   1. Check logs: kubectl logs -f -l app=gitea-service -n dev-platform -c gitea-service"
echo "   2. Check proxy: kubectl logs -f -l app=gitea-service -n dev-platform -c istio-proxy"
echo "   3. Verify mTLS: istioctl authn tls-check POD_NAME -n dev-platform"
echo "   4. View in Kiali: istioctl dashboard kiali"
