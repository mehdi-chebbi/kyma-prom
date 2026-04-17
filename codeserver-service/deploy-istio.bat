@echo off
echo Building and deploying CodeServer Service with Istio...
echo.

echo Step 1: Building Docker image...
nerdctl build -t codeserver-service .
if errorlevel 1 (
    echo Failed to build Docker image
    exit /b 1
)

echo Step 2: Saving Docker image to tar...
nerdctl save -o codeserver-service.tar codeserver-service
if errorlevel 1 (
    echo Failed to save Docker image
    exit /b 1
)
echo Image saved successfully

echo Step 3: Loading image into Kubernetes...
nerdctl load --namespace k8s.io -i codeserver-service.tar
if errorlevel 1 (
    echo Failed to load Docker image
    exit /b 1
)
echo Image loaded successfully
echo.

echo Step 4: Enabling Istio injection on dev-platform namespace...
kubectl label namespace dev-platform istio-injection=enabled --overwrite
echo.

echo Step 5: Creating codeserver-instances namespace if not exists...
kubectl create namespace codeserver-instances --dry-run=client -o yaml | kubectl apply -f -
kubectl label namespace codeserver-instances istio-injection=enabled --overwrite
echo.

echo Step 6: Deleting existing deployment...
kubectl delete deployment codeserver-service -n dev-platform --ignore-not-found=true
echo.

echo Step 7: Applying Kubernetes manifests...
kubectl apply -f .\k8s\00-namespace.yaml
kubectl apply -f .\k8s\01-configmap.yaml
kubectl apply -f .\k8s\02-secret.yaml
kubectl apply -f .\k8s\03-rbac.yaml
kubectl apply -f .\k8s\04-deployment.yaml
kubectl apply -f .\k8s\05-service.yaml
echo.

echo Step 8: Applying Istio configuration...
kubectl apply -f .\k8s\06-istio-config.yaml
echo.

echo Step 9: Waiting for deployment to be ready...
kubectl rollout status deployment/codeserver-service -n dev-platform --timeout=120s
echo.

echo Step 10: Verifying deployment...
echo.
echo === Pods ===
kubectl get pods -n dev-platform -l app=codeserver-service
echo.
echo === Service ===
kubectl get svc codeserver-service -n dev-platform
echo.
echo === Istio Gateway ===
kubectl get gateway -n dev-platform -l app=codeserver-service
echo.
echo === Istio VirtualService ===
kubectl get virtualservice -n dev-platform
echo.

echo ========================================
echo Deployment complete!
echo ========================================
echo.
echo Service exposed on LoadBalancer port 30012
echo Access via: http://localhost:30012
echo.
echo To check logs:
echo   kubectl logs -f -l app=codeserver-service -n dev-platform -c codeserver-service
echo.
echo To check Istio proxy logs:
echo   kubectl logs -f -l app=codeserver-service -n dev-platform -c istio-proxy
echo.
