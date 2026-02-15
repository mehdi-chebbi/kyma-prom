@echo off
echo Building and deploying Gitea Sync Controller...
echo.

echo Step 1: Building Docker image...
nerdctl build -t gitea-sync-controller -f Dockerfile.controller .
if errorlevel 1 (
    echo Failed to build Docker image
    exit /b 1
)

echo Step 2: Saving Docker image to tar...
nerdctl save -o gitea-sync-controller.tar gitea-sync-controller
if errorlevel 1 (
    echo Failed to save Docker image
    exit /b 1
)
echo Image saved successfully

echo Step 3: Loading image into Kubernetes...
nerdctl load --namespace k8s.io -i gitea-sync-controller.tar
if errorlevel 1 (
    echo Failed to load Docker image
    exit /b 1
)
echo Image loaded successfully
echo.

echo Step 4: Creating dev-platform namespace if not exists...
kubectl create namespace dev-platform 2>nul
echo.

echo Step 5: Deleting existing controller (if any)...
kubectl delete statefulset gitea-sync-controller -n dev-platform 2>nul
echo.

echo Step 6: Applying Kubernetes manifests...
kubectl apply -f .\k8s\01-configmap.yaml
kubectl apply -f .\k8s\02-secret.yaml
kubectl apply -f .\k8s\07-controller-deployment.yaml
echo.

echo Step 7: Waiting for controller to be ready...
kubectl rollout status statefulset/gitea-sync-controller -n dev-platform --timeout=120s
echo.

echo Step 8: Verifying deployment...
echo.
echo === Controller Pod ===
kubectl get pods -n dev-platform -l app=gitea-sync-controller
echo.
echo === Controller Service ===
kubectl get svc gitea-sync-controller -n dev-platform
echo.
echo === PVC ===
kubectl get pvc -n dev-platform -l app=gitea-sync-controller
echo.

echo ========================================
echo Controller deployment complete!
echo ========================================
echo.
echo Controller runs 4 goroutines:
echo   1. Reconcile loop (Gitea repos to LDAP)
echo   2. Webhook health check
echo   3. Retry queue processor
echo   4. Group sync (LDAP groups/depts to Gitea teams)
echo.
echo To check logs:
echo   kubectl logs -f -l app=gitea-sync-controller -n dev-platform
echo.
echo To check health:
echo   kubectl exec -n dev-platform statefulset/gitea-sync-controller -- wget -qO- http://localhost:8081/health
echo.
