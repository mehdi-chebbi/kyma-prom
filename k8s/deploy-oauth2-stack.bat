@echo off
setlocal enabledelayedexpansion

echo ================================================================
echo   Deploying Complete OAuth2 Stack with Keycloak + Gitea
echo ================================================================
echo.

REM Check prerequisites
echo Checking prerequisites...
where kubectl >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo ERROR: kubectl not found. Please install kubectl.
    exit /b 1
)

echo.
echo ================================================================
echo   PHASE 1: Deploy Authentication Infrastructure (Keycloak)
echo ================================================================
echo.

cd auth

echo [1/7] Creating auth-system namespace...
kubectl apply -f 01-namespace.yaml

echo.
echo [2/7] Deploying PostgreSQL (shared database)...
kubectl apply -f 02-postgres.yaml
echo Waiting for PostgreSQL...
kubectl wait --for=condition=ready pod -l app=postgres -n auth-system --timeout=300s

echo.
echo [3/7] Deploying Memcached (session cache)...
kubectl apply -f 03-memcached.yaml
echo Waiting for Memcached...
kubectl wait --for=condition=ready pod -l app=memcached -n auth-system --timeout=120s

echo.
echo [4/7] Deploying Keycloak...
kubectl apply -f 04-keycloak.yaml
echo Waiting for Keycloak (this may take 2-3 minutes)...
kubectl wait --for=condition=ready pod -l app=keycloak -n auth-system --timeout=300s

echo.
echo [5/7] Deploying Keycloak Ingress...
kubectl apply -f 05-keycloak-ingress.yaml

echo.
echo [6/7] Deploying Istio Authentication Policies...
kubectl apply -f 06-istio-auth.yaml

echo.
echo [7/7] Configuring Keycloak LDAP Federation...
kubectl delete job keycloak-ldap-config -n auth-system --ignore-not-found=true
kubectl apply -f 07-keycloak-ldap-config.yaml
echo Waiting for LDAP configuration...
kubectl wait --for=condition=complete job/keycloak-ldap-config -n auth-system --timeout=180s

cd ..

echo.
echo ================================================================
echo   PHASE 2: Deploy Gitea (Resource Server)
echo ================================================================
echo.

cd dev-platform

echo [1/3] Initializing Gitea database in PostgreSQL...
kubectl apply -f gitea-deployment.yaml
echo Waiting for database initialization...
kubectl wait --for=condition=complete job/gitea-db-init -n dev-platform --timeout=120s

echo.
echo [2/3] Deploying Gitea application...
echo Waiting for Gitea to be ready...
kubectl wait --for=condition=ready pod -l app=gitea -n dev-platform --timeout=300s

echo.
echo [3/3] Configuring Gitea OAuth2 with Keycloak...
kubectl apply -f gitea-oauth2-config.yaml
kubectl delete job gitea-oauth2-setup -n dev-platform --ignore-not-found=true
kubectl apply -f gitea-oauth2-config.yaml
echo Waiting for OAuth2 configuration...
timeout /t 10 /nobreak > nul
kubectl wait --for=condition=complete job/gitea-oauth2-setup -n dev-platform --timeout=120s || echo OAuth2 setup may need manual configuration

cd ..

echo.
echo ================================================================
echo   PHASE 3: Deployment Status
echo ================================================================
echo.

echo Pods in auth-system:
kubectl get pods -n auth-system

echo.
echo Pods in dev-platform:
kubectl get pods -n dev-platform

echo.
echo Services:
kubectl get svc -n auth-system
kubectl get svc -n dev-platform

echo.
echo Istio Policies:
kubectl get requestauthentication,authorizationpolicy -n istio-system
kubectl get authorizationpolicy -n dev-platform

echo.
echo ================================================================
echo   ACCESS INFORMATION
echo ================================================================
echo.

echo Keycloak Admin Console:
echo   URL: http://keycloak.localhost
echo   Username: admin
echo   Password: Run this command to get password:
echo   kubectl get secret keycloak-admin-secret -n auth-system -o jsonpath="{.data.KEYCLOAK_ADMIN_PASSWORD}"
echo.

echo Gitea:
echo   URL: http://gitea.localhost
echo   Login via Keycloak SSO
echo.

echo Keycloak Realm:
echo   Realm: devplatform
echo   Token Endpoint: http://keycloak.localhost/realms/devplatform/protocol/openid-connect/token
echo.

echo ================================================================
echo   TESTING THE OAUTH2 FLOW
echo ================================================================
echo.

echo Step 1: Get JWT Token from Keycloak
echo ----------------------------------------
echo curl -X POST http://keycloak.localhost/realms/devplatform/protocol/openid-connect/token \
echo   -H "Content-Type: application/x-www-form-urlencoded" \
echo   -d "grant_type=password" \
echo   -d "client_id=gitea-service" \
echo   -d "client_secret=<get-from-secret>" \
echo   -d "username=john.doe" \
echo   -d "password=password123"
echo.

echo Get client secret:
echo   kubectl get secret keycloak-client-secrets -n auth-system -o jsonpath="{.data.gitea-service-secret}"
echo.

echo Step 2: Use Token in GraphQL Request
echo ----------------------------------------
echo curl -X POST http://gitea-service.localhost/graphql \
echo   -H "Authorization: Bearer <access_token>" \
echo   -H "Content-Type: application/json" \
echo   -d "{\"query\":\"{ listRepositories { items { name } } }\"}"
echo.

echo Step 3: Verify Token Flow
echo ----------------------------------------
echo 1. Istio validates JWT at gateway
echo 2. Adds X-Forwarded-User header
echo 3. gitea-service extracts user from headers
echo 4. Passes same token to Gitea API
echo 5. Gitea validates JWT with Keycloak
echo.

echo ================================================================
echo   DEPLOYMENT COMPLETE!
echo ================================================================
echo.

echo Next Steps:
echo 1. Access Keycloak: kubectl port-forward -n auth-system svc/keycloak 8080:8080
echo 2. Access Gitea: kubectl port-forward -n dev-platform svc/gitea 3000:3000
echo 3. Test authentication flow using the commands above
echo 4. Check OAUTH2_FLOW.md for detailed documentation
echo.

echo For logs:
echo   Keycloak: kubectl logs -n auth-system -l app=keycloak -f
echo   Gitea: kubectl logs -n dev-platform -l app=gitea -f
echo.

pause
