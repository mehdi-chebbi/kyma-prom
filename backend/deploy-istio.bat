@echo off
echo Building and deploying LDAP Manager Service with Istio...
echo.

echo Step 1: Building Docker image...
nerdctl build -t ldap-manager .
if errorlevel 1 (
    echo Failed to build Docker image
    exit /b 1
)

echo Step 2: Saving Docker image to tar...
nerdctl save -o ldap-manager.tar ldap-manager
if errorlevel 1 (
    echo Failed to save Docker image
    exit /b 1
)
echo Image saved successfully

echo Step 3: Loading image into Kubernetes...
nerdctl load --namespace k8s.io -i ldap-manager.tar
if errorlevel 1 (
    echo Failed to load Docker image
    exit /b 1
)
echo Image loaded successfully
echo.

echo Step 4: Enabling Istio injection on dev-platform namespace...
kubectl label namespace dev-platform istio-injection=enabled --overwrite
echo.

echo Step 5: Deploying OpenLDAP if not exists...
kubectl apply -f .\k8s\01-openldap.yaml
echo.

echo Step 6: Waiting for OpenLDAP to be ready...
kubectl wait --for=condition=ready pod -l app=openldap1 -n dev-platform --timeout=180s
if errorlevel 1 (
    echo OpenLDAP not ready, continuing anyway...
)
echo.

echo Step 7: Deleting existing LDAP Manager deployment...
kubectl delete deployment ldap-manager -n dev-platform
echo.

echo Step 8: Applying LDAP Manager Kubernetes manifests...
kubectl apply -f .\k8s\03-ldap-manager.yaml
echo.

echo Step 9: Applying Istio configuration...
kubectl apply -f .\k8s\04-istio-config.yaml
echo.

echo Step 10: Waiting for deployment to be ready...
kubectl rollout status deployment/ldap-manager -n dev-platform --timeout=120s
echo.

echo Step 11: Verifying deployment...
echo.
echo === Pods ===
kubectl get pods -n dev-platform -l app=ldap-manager
echo.
echo === Service ===
kubectl get svc ldap-manager -n dev-platform
echo.
echo === Istio Gateway ===
kubectl get gateway -n dev-platform
echo.
echo === Istio VirtualService ===
kubectl get virtualservice -n dev-platform
echo.

echo ========================================
echo Deployment complete!
echo ========================================
echo.
echo Service exposed on LoadBalancer port 30008
echo Access via: http://localhost:30008
echo.
echo Test login with:
echo   Username: john.doe
echo   Password: password123
echo.
echo To check logs:
echo   kubectl logs -f -l app=ldap-manager -n dev-platform -c ldap-manager
echo.
echo To check Istio proxy logs:
echo   kubectl logs -f -l app=ldap-manager -n dev-platform -c istio-proxy
echo.
