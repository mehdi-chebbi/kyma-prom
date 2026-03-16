@echo off
setlocal enabledelayedexpansion

echo ============================================
echo   Deploying kyma-flow CLI
echo ============================================
echo.

REM Check if kubectl is available
where kubectl >nul 2>&1 || (
    echo ERROR: kubectl not found in PATH
    echo Please install kubectl and add it to your PATH
    exit /b 1
)

REM Check if we can connect to the cluster
kubectl cluster-info >nul 2>&1 || (
    echo ERROR: Cannot connect to Kubernetes cluster
    echo Please check your kubeconfig
    exit /b 1
)

REM Get script directory
set SCRIPT_DIR=%~dp0
set CONFIGMAP_FILE=%SCRIPT_DIR%kyma-flow-configmap.yaml
set SERVICES_FILE=%SCRIPT_DIR%metrics-services.yaml

REM Check if files exist
if not exist "%CONFIGMAP_FILE%" (
    echo ERROR: ConfigMap file not found: %CONFIGMAP_FILE%
    exit /b 1
)

if not exist "%SERVICES_FILE%" (
    echo ERROR: Services file not found: %SERVICES_FILE%
    exit /b 1
)

echo [1/4] Applying Metrics Services...
kubectl apply -f "%SERVICES_FILE%"
if %ERRORLEVEL% NEQ 0 (
    echo ERROR: Failed to apply metrics services
    exit /b 1
)
echo Metrics services applied successfully
echo.

echo [2/4] Applying ConfigMap...
kubectl apply -f "%CONFIGMAP_FILE%"
if %ERRORLEVEL% NEQ 0 (
    echo ERROR: Failed to apply ConfigMap
    exit /b 1
)
echo ConfigMap applied successfully
echo.

echo [3/4] Restarting CodeServer Service...
kubectl rollout restart deployment/codeserver-service -n dev-platform
if %ERRORLEVEL% NEQ 0 (
    echo ERROR: Failed to restart deployment
    exit /b 1
)
echo Rollout initiated successfully
echo.

echo [4/4] Waiting for rollout to complete...
kubectl rollout status deployment/codeserver-service -n dev-platform --timeout=120s
if %ERRORLEVEL% NEQ 0 (
    echo WARNING: Rollout did not complete within timeout
    echo Check status with: kubectl get pods -n dev-platform
) else (
    echo Rollout completed successfully
)
echo.

echo ============================================
echo   kyma-flow CLI deployed successfully!
echo ============================================
echo.
echo To use kyma-flow:
echo   1. Enter a CodeServer pod:
echo      kubectl exec -it deployment/codeserver-service -n dev-platform -- sh
echo.
echo   2. Run commands:
echo      /opt/kyma-flow/kyma-flow user
echo      /opt/kyma-flow/kyma-flow summary
echo.
echo   3. (Optional) Create an alias:
echo      alias kyma-flow='/opt/kyma-flow/kyma-flow'
echo.
echo Available commands:
echo   user, group, repo, cloned-repo, active-workspace, storage, summary, help
echo.

endlocal
