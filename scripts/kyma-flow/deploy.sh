#!/bin/bash
#
# deploy.sh - Deploy kyma-flow CLI to the cluster
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIGMAP_FILE="${SCRIPT_DIR}/kyma-flow-configmap.yaml"

echo "============================================"
echo "  Deploying kyma-flow CLI"
echo "============================================"
echo ""

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "❌ Error: kubectl is not installed or not in PATH"
    exit 1
fi

# Check if we can connect to the cluster
if ! kubectl cluster-info &> /dev/null; then
    echo "❌ Error: Cannot connect to Kubernetes cluster"
    exit 1
fi

# Check if ConfigMap file exists
if [ ! -f "$CONFIGMAP_FILE" ]; then
    echo "❌ Error: ConfigMap file not found: $CONFIGMAP_FILE"
    exit 1
fi

echo "[1/3] Applying ConfigMap..."
kubectl apply -f "$CONFIGMAP_FILE"
echo "✅ ConfigMap applied"
echo ""

echo "[2/3] Restarting CodeServer Service..."
kubectl rollout restart deployment/codeserver-service -n dev-platform
echo "✅ Rollout initiated"
echo ""

echo "[3/3] Waiting for rollout to complete..."
kubectl rollout status deployment/codeserver-service -n dev-platform --timeout=120s
echo "✅ Rollout complete"
echo ""

echo "============================================"
echo "  kyma-flow CLI deployed successfully!"
echo "============================================"
echo ""
echo "To use kyma-flow:"
echo "  1. Enter a CodeServer pod:"
echo "     kubectl exec -it deployment/codeserver-service -n dev-platform -- bash"
echo ""
echo "  2. Run commands:"
echo "     /opt/kyma-flow/kyma-flow user"
echo "     /opt/kyma-flow/kyma-flow summary"
echo ""
echo "  3. (Optional) Create an alias:"
echo "     alias kyma-flow='/opt/kyma-flow/kyma-flow'"
echo ""
echo "Available commands:"
echo "  user, group, repo, cloned-repo, active-workspace, storage, summary, help"
echo ""
