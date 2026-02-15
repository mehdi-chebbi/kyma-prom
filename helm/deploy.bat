@echo off
setlocal enabledelayedexpansion

echo ============================================
echo   DevPlatform Helm Deployment
echo ============================================
echo.

REM Check prerequisites
where helm >nul 2>&1 || (echo ERROR: helm not found in PATH && exit /b 1)
where nerdctl >nul 2>&1 || (echo ERROR: nerdctl not found in PATH && exit /b 1)
where kubectl >nul 2>&1 || (echo ERROR: kubectl not found in PATH && exit /b 1)

set SCRIPT_DIR=%~dp0
set PROJECT_DIR=%SCRIPT_DIR%..

echo [1/5] Building Docker images...
echo.

echo Building ldap-manager...
cd /d "%PROJECT_DIR%\backend"
nerdctl build -t ldap-manager:latest . || (echo ERROR: ldap-manager build failed && exit /b 1)

echo Building gitea-service...
cd /d "%PROJECT_DIR%\gitea-service"
nerdctl build -t gitea-service:latest . || (echo ERROR: gitea-service build failed && exit /b 1)

echo Building gitea-sync-controller...
nerdctl build -t gitea-sync-controller:latest -f Dockerfile.controller . || (echo ERROR: gitea-sync-controller build failed && exit /b 1)

echo Building codeserver-service...
cd /d "%PROJECT_DIR%\codeserver-service"
nerdctl build -t codeserver-service:latest . || (echo ERROR: codeserver-service build failed && exit /b 1)

echo Building ldap-init...
cd /d "%SCRIPT_DIR%devplatform\charts\openldap\init-container"
nerdctl build -t ldap-init:latest . || (echo ERROR: ldap-init build failed && exit /b 1)

echo.
echo [2/5] Loading images into k8s namespace...
echo.

for %%I in (ldap-manager gitea-service gitea-sync-controller codeserver-service ldap-init) do (
    echo Loading %%I:latest into k8s namespace...
    nerdctl save %%I:latest | nerdctl --namespace k8s.io load || (echo ERROR: failed to load %%I && exit /b 1)
)

echo.
echo [3/5] Installing Helm chart...
echo.

cd /d "%SCRIPT_DIR%"
@REM for first time run of helm chart delete previous roles
@REM kubectl delete clusterrole codeserver-manager 
@REM kubectl delete clusterrolebinding codeserver-service-binding
helm upgrade --install devplatform ./devplatform -f ./devplatform/values-dev.yaml --timeout 10m || (echo ERROR: helm install failed && exit /b 1)

echo.
echo [4/5] Waiting for deployments to be ready...
echo.

echo Waiting for OpenLDAP...
kubectl rollout status statefulset/openldap -n dev-platform --timeout=120s 2>nul

echo Waiting for PostgreSQL...
kubectl rollout status statefulset/postgres -n auth-system --timeout=120s 2>nul

echo Waiting for Keycloak...
kubectl rollout status statefulset/keycloak -n auth-system --timeout=300s 2>nul

echo Waiting for LDAP Manager...
kubectl rollout status deployment/ldap-manager -n dev-platform --timeout=120s 2>nul

echo Waiting for Gitea...
kubectl rollout status deployment/gitea -n dev-platform --timeout=300s 2>nul

echo Waiting for Gitea Service...
kubectl rollout status deployment/gitea-service -n dev-platform --timeout=120s 2>nul

echo Waiting for CodeServer Service...
kubectl rollout status deployment/codeserver-service -n dev-platform --timeout=120s 2>nul

echo.
echo [5/5] Deployment status
echo.

echo === auth-system ===
kubectl get pods -n auth-system
echo.
echo === dev-platform ===
kubectl get pods -n dev-platform
echo.
echo === codeserver-instances ===
kubectl get pods -n codeserver-instances 2>nul
echo.
echo === Services ===
kubectl get svc -n dev-platform
kubectl get svc -n auth-system
echo.

echo ============================================
echo   Deployment Complete!
echo ============================================
echo.
echo Access Points:
echo   Keycloak:         http://localhost:30080
echo   LDAP Manager:     http://localhost:30008/graphql
echo   Gitea:            http://localhost:30009
echo   Gitea Service:    http://localhost:30011/graphql
echo   CodeServer:       http://localhost:30012/graphql
echo.
echo Helm Commands:
echo   Status:    helm status devplatform
echo   Upgrade:   helm upgrade devplatform ./devplatform -f ./devplatform/values-dev.yaml
echo   Rollback:  helm rollback devplatform
echo   Uninstall: helm uninstall devplatform
echo.

endlocal
