@echo off
setlocal enabledelayedexpansion

echo === CodeServer Service Deployment ===

cd /d "%~dp0"

echo Building Docker image with nerdctl...
nerdctl build -t codeserver-service:latest .
if %errorlevel% neq 0 (
    echo Failed to build image
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

echo Creating namespaces...
kubectl apply -f k8s\00-namespace.yaml

echo Deploying ConfigMap and Secrets...
kubectl apply -f k8s\01-configmap.yaml
kubectl apply -f k8s\02-secret.yaml

echo Setting up RBAC...
kubectl apply -f k8s\03-rbac.yaml

echo Deploying service...
kubectl apply -f k8s\04-deployment.yaml
kubectl apply -f k8s\05-service.yaml

echo Configuring Istio...
kubectl apply -f k8s\06-istio-config.yaml

echo Waiting for deployment...
kubectl rollout status deployment/codeserver-service -n dev-platform --timeout=120s

echo.
echo === Deployment Complete ===
echo.
kubectl get pods -l app=codeserver-service -n dev-platform
echo.
echo Service URL: http://codeserver.devplatform.local/graphql
echo Health: http://localhost:30012/health
echo.
echo To port-forward: kubectl port-forward svc/codeserver-service 8082:30012 -n dev-platform

pause
