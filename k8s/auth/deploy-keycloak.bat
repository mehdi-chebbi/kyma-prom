@echo off
setlocal enabledelayedexpansion

echo =========================================
echo Deploying Keycloak Authentication System
echo =========================================

REM Check prerequisites
echo Checking prerequisites...
where kubectl >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo kubectl not found. Please install kubectl.
    exit /b 1
)

REM Step 1: Create namespace
echo.
echo [1/7] Creating auth-system namespace...
kubectl apply -f 01-namespace.yaml

REM Step 2: Deploy PostgreSQL
echo.
echo [2/7] Deploying PostgreSQL...
kubectl apply -f 02-postgres.yaml

echo Waiting for PostgreSQL to be ready...
kubectl wait --for=condition=ready pod -l app=postgres -n auth-system --timeout=300s

REM Step 3: Deploy Memcached
echo.
echo [3/7] Deploying Memcached...
kubectl apply -f 03-memcached.yaml

echo Waiting for Memcached to be ready...
kubectl wait --for=condition=ready pod -l app=memcached -n auth-system --timeout=120s

REM Step 4: Deploy Keycloak
echo.
echo [4/7] Deploying Keycloak...
kubectl apply -f 04-keycloak.yaml

echo Waiting for Keycloak to be ready (this may take 2-3 minutes)...
kubectl wait --for=condition=ready pod -l app=keycloak -n auth-system --timeout=300s

REM Step 5: Deploy Ingress
echo.
echo [5/7] Deploying Keycloak Ingress...
kubectl apply -f 05-keycloak-ingress.yaml

REM Step 6: Deploy Istio Authentication
echo.
echo [6/7] Deploying Istio Authentication Policies...
kubectl apply -f 06-istio-auth.yaml

REM Step 7: Configure LDAP Federation
echo.
echo [7/7] Configuring Keycloak LDAP Federation...
kubectl delete job keycloak-ldap-config -n auth-system --ignore-not-found=true
kubectl apply -f 07-keycloak-ldap-config.yaml

echo Waiting for LDAP configuration job to complete...
kubectl wait --for=condition=complete job/keycloak-ldap-config -n auth-system --timeout=180s

REM Show status
echo.
echo =========================================
echo Deployment Status
echo =========================================

echo.
echo Pods in auth-system:
kubectl get pods -n auth-system

echo.
echo Services in auth-system:
kubectl get svc -n auth-system

echo.
echo Istio Authentication Policies:
kubectl get requestauthentication,authorizationpolicy -n istio-system
kubectl get authorizationpolicy -n dev-platform

REM Access information
echo.
echo =========================================
echo Access Information
echo =========================================

echo.
echo Keycloak Admin Console:
echo   URL: http://keycloak.localhost
echo   Username: admin
echo   Password: Check secret 'keycloak-admin-secret' in auth-system namespace

echo.
echo Get admin password:
echo   kubectl get secret keycloak-admin-secret -n auth-system -o jsonpath="{.data.KEYCLOAK_ADMIN_PASSWORD}"

echo.
echo Keycloak Realm:
echo   Realm: devplatform
echo   Token Endpoint: http://keycloak.auth-system:8080/realms/devplatform/protocol/openid-connect/token
echo   JWKS Endpoint: http://keycloak.auth-system:8080/realms/devplatform/protocol/openid-connect/certs

echo.
echo Test Token Acquisition:
echo   curl -X POST http://keycloak.localhost/realms/devplatform/protocol/openid-connect/token ^
echo     -H "Content-Type: application/x-www-form-urlencoded" ^
echo     -d "grant_type=password" ^
echo     -d "client_id=gitea-service" ^
echo     -d "client_secret=<client-secret>" ^
echo     -d "username=john.doe" ^
echo     -d "password=password123"

echo.
echo =========================================
echo Deployment Complete!
echo =========================================

echo.
echo To access Keycloak locally, run:
echo   kubectl port-forward -n auth-system svc/keycloak 8080:8080
echo   Then access: http://localhost:8080

pause
