@echo off
setlocal enabledelayedexpansion

:: ============================================================
::  deploy-harbor.bat
::  Deploys Harbor (ClusterIP) and automates full setup
::  Harbor API calls are made via a temp pod inside the cluster
:: ============================================================

set HARBOR_NAMESPACE=harbor
set HARBOR_ADMIN_PASS=Harbor12345
set HARBOR_SVC=harbor.harbor.svc.cluster.local
set HARBOR_API=http://%HARBOR_SVC%/api/v2.0
set PROJECT_NAME=devplatform
set ROBOT_NAME=kaniko-pusher
set K8S_SECRET_NAME=harbor-credentials
set K8S_NAMESPACE=dev-platform
set VALUES_FILE=%~dp0harbor-values.yaml
set CREDENTIALS_FILE=%~dp0harbor-robot-credentials.txt
set TMP_POD=harbor-setup-tmp

echo.
echo ============================================================
echo   Harbor Registry Deployment
echo ============================================================
echo   Values file : %VALUES_FILE%
echo   Namespace   : %HARBOR_NAMESPACE%
echo   Project     : %PROJECT_NAME%
echo   K8s secret  : %K8S_SECRET_NAME% in %K8S_NAMESPACE%
echo ============================================================
echo.

:: Verify values file exists
if not exist "%VALUES_FILE%" (
    echo [ERROR] harbor-values.yaml not found at: %VALUES_FILE%
    echo [ERROR] Place harbor-values.yaml in the same folder as this script.
    exit /b 1
)

:: ============================================================
:: [1/7] Add Harbor Helm repo and update
:: ============================================================
echo [1/7] Adding Harbor Helm repository...
helm repo add harbor https://helm.goharbor.io 2>nul
if errorlevel 1 (
    echo [1/7] Repo already exists or warning - continuing.
)
echo [1/7] Updating Helm repositories...
helm repo update
if errorlevel 1 (
    echo [ERROR] Failed to update Helm repos.
    exit /b 1
)
echo [1/7] Done.
echo.

:: ============================================================
:: [2/7] Create namespace and deploy Harbor via Helm
:: ============================================================
echo [2/7] Creating namespace '%HARBOR_NAMESPACE%'...
kubectl create namespace %HARBOR_NAMESPACE% 2>nul
if errorlevel 1 (
    echo [2/7] Namespace already exists, skipping.
)

echo [2/7] Deploying Harbor via Helm with harbor-values.yaml...
helm install harbor harbor/harbor ^
  --namespace %HARBOR_NAMESPACE% ^
  --values "%VALUES_FILE%"

if errorlevel 1 (
    echo [2/7] Helm install returned non-zero - Harbor may already be installed.
    echo [2/7] Attempting upgrade instead...
    helm upgrade harbor harbor/harbor ^
      --namespace %HARBOR_NAMESPACE% ^
      --values "%VALUES_FILE%"
    if errorlevel 1 (
        echo [ERROR] Both install and upgrade failed. Check helm status.
        helm status harbor -n %HARBOR_NAMESPACE%
        exit /b 1
    )
)
echo [2/7] Done.
echo.

:: ============================================================
:: [3/7] Wait for Harbor to be ready
:: ============================================================
echo [3/7] Waiting for harbor-core deployment (timeout: 300s)...
kubectl rollout status deployment/harbor-core -n %HARBOR_NAMESPACE% --timeout=300s
if errorlevel 1 (
    echo [ERROR] harbor-core not ready. Check pods:
    kubectl get pods -n %HARBOR_NAMESPACE%
    exit /b 1
)

echo [3/7] Waiting for harbor-registry deployment (timeout: 300s)...
kubectl rollout status deployment/harbor-registry -n %HARBOR_NAMESPACE% --timeout=300s
if errorlevel 1 (
    echo [ERROR] harbor-registry not ready. Check pods:
    kubectl get pods -n %HARBOR_NAMESPACE%
    exit /b 1
)
echo [3/7] Harbor is ready.
echo.

:: ============================================================
:: [4/7] Launch temp pod and create devplatform project
:: ============================================================
echo [4/7] Launching temporary curl pod for API calls (Harbor is ClusterIP)...

:: Clean up any leftover temp pod from a previous run
kubectl delete pod %TMP_POD% -n %HARBOR_NAMESPACE% --ignore-not-found 2>nul
timeout /t 3 /nobreak >nul

kubectl run %TMP_POD% ^
  --image=curlimages/curl:latest ^
  --restart=Never ^
  --namespace=%HARBOR_NAMESPACE% ^
  --command -- sleep 300

echo [4/7] Waiting for temp pod to be ready...
kubectl wait pod/%TMP_POD% ^
  --for=condition=Ready ^
  --namespace=%HARBOR_NAMESPACE% ^
  --timeout=60s
if errorlevel 1 (
    echo [ERROR] Temp pod failed to start.
    kubectl describe pod %TMP_POD% -n %HARBOR_NAMESPACE%
    exit /b 1
)

echo [4/7] Creating project '%PROJECT_NAME%' via Harbor API...
kubectl exec %TMP_POD% -n %HARBOR_NAMESPACE% -- ^
  curl -s -o /dev/null -w "%%{http_code}" ^
  -u admin:%HARBOR_ADMIN_PASS% ^
  -X POST "%HARBOR_API%/projects" ^
  -H "Content-Type: application/json" ^
  -d "{\"project_name\":\"%PROJECT_NAME%\",\"public\":false}" > %TEMP%\harbor_proj_status.txt 2>&1

set /p PROJ_STATUS=<%TEMP%\harbor_proj_status.txt
echo [4/7] API response: HTTP %PROJ_STATUS%

if "%PROJ_STATUS%"=="201" (
    echo [4/7] Project '%PROJECT_NAME%' created successfully.
) else if "%PROJ_STATUS%"=="409" (
    echo [4/7] Project '%PROJECT_NAME%' already exists - continuing.
) else (
    echo [WARN] Unexpected response %PROJ_STATUS% from project API - check Harbor logs.
)
echo.

:: ============================================================
:: [5/7] Create robot account, capture name and secret
:: ============================================================
echo [5/7] Getting project ID for '%PROJECT_NAME%'...

kubectl exec %TMP_POD% -n %HARBOR_NAMESPACE% -- ^
  curl -s -u admin:%HARBOR_ADMIN_PASS% ^
  "%HARBOR_API%/projects/%PROJECT_NAME%" ^
  -H "X-Is-Resource-Name: true" > %TEMP%\harbor_project.json 2>&1

:: Parse project ID
for /f "delims=" %%i in ('powershell -NoProfile -Command ^
  "try { $j = Get-Content '%TEMP%\harbor_project.json' -Raw | ConvertFrom-Json; $j.project_id } catch { exit 1 }"') do set PROJECT_ID=%%i

if "%PROJECT_ID%"=="" (
    echo [ERROR] Could not get project ID. Check if project '%PROJECT_NAME%' exists.
    kubectl delete pod %TMP_POD% -n %HARBOR_NAMESPACE% --ignore-not-found >nul 2>&1
    exit /b 1
)

echo [5/7] Project ID: %PROJECT_ID%

echo [5/7] Checking if robot '%ROBOT_NAME%' already exists...

kubectl exec %TMP_POD% -n %HARBOR_NAMESPACE% -- ^
  curl -s -u admin:%HARBOR_ADMIN_PASS% ^
  "%HARBOR_API%/robots?q=Level=project,ProjectID=%PROJECT_ID%" > %TEMP%\harbor_robots_list.json 2>&1

:: Check if robot with our name exists in the list
for /f "delims=" %%i in ('powershell -NoProfile -Command ^
  "try { $n = '%ROBOT_NAME%'; $j = Get-Content '%TEMP%\harbor_robots_list.json' -Raw | ConvertFrom-Json; $j | Where-Object { $_.name -like ('*' + $n) } | Select-Object -First 1 -ExpandProperty id } catch { '' }"') do set EXISTING_ROBOT_ID=%%i

if "%EXISTING_ROBOT_ID%" NEQ "" (
    echo [5/7] Robot already exists (ID: %EXISTING_ROBOT_ID%), deleting it...
    kubectl exec %TMP_POD% -n %HARBOR_NAMESPACE% -- ^
      curl -s -u admin:%HARBOR_ADMIN_PASS% ^
      -X DELETE "%HARBOR_API%/robots/%EXISTING_ROBOT_ID%" > %TEMP%\harbor_delete.json 2>&1
    echo [5/7] Existing robot deleted.
)

echo [5/7] Creating robot account '%ROBOT_NAME%'...

kubectl exec %TMP_POD% -n %HARBOR_NAMESPACE% -- ^
  curl -s ^
  -u admin:%HARBOR_ADMIN_PASS% ^
  -X POST "%HARBOR_API%/robots" ^
  -H "Content-Type: application/json" ^
  -d "{\"name\":\"%ROBOT_NAME%\",\"description\":\"Robot account for Kaniko image builds\",\"duration\":-1,\"level\":\"project\",\"permissions\":[{\"kind\":\"project\",\"namespace\":\"%PROJECT_NAME%\",\"access\":[{\"resource\":\"repository\",\"action\":\"push\"},{\"resource\":\"repository\",\"action\":\"pull\"},{\"resource\":\"artifact\",\"action\":\"read\"}]}]}" > %TEMP%\harbor_robot.json 2>&1

echo [5/7] Raw robot API response:
type %TEMP%\harbor_robot.json
echo.

:: Parse robot name from JSON response using PowerShell
for /f "delims=" %%i in ('powershell -NoProfile -Command ^
  "try { $j = Get-Content '%TEMP%\harbor_robot.json' -Raw | ConvertFrom-Json; $j.name } catch { exit 1 }"') do set ROBOT_FULL_NAME=%%i

:: Parse robot secret from JSON response using PowerShell
for /f "delims=" %%i in ('powershell -NoProfile -Command ^
  "try { $j = Get-Content '%TEMP%\harbor_robot.json' -Raw | ConvertFrom-Json; $j.secret } catch { exit 1 }"') do set ROBOT_SECRET=%%i

:: Validate we got both values
if "%ROBOT_FULL_NAME%"=="" (
    echo [ERROR] Could not parse robot name from API response.
    echo [ERROR] Check the raw response above for error details.
    kubectl delete pod %TMP_POD% -n %HARBOR_NAMESPACE% --ignore-not-found >nul 2>&1
    exit /b 1
)
if "%ROBOT_SECRET%"=="" (
    echo [ERROR] Could not parse robot secret from API response.
    kubectl delete pod %TMP_POD% -n %HARBOR_NAMESPACE% --ignore-not-found >nul 2>&1
    exit /b 1
)

echo [5/7] Robot account created:
echo        Name   : %ROBOT_FULL_NAME%
echo        Secret : %ROBOT_SECRET%
echo.

echo [5/7] Cleaning up temp pod...
kubectl delete pod %TMP_POD% -n %HARBOR_NAMESPACE% --ignore-not-found >nul 2>&1

:: ============================================================
:: [6/7] Create Kubernetes docker-registry secret (generic type with config.json key for Kaniko)
:: Note: Kaniko expects config.json key, not .dockerconfigjson
:: ============================================================
set INSTANCE_NAMESPACE=codeserver-instances

:: Create auth string (username:password) and encode to base64
for /f "delims=" %%i in ('powershell -NoProfile -Command "[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes('%ROBOT_FULL_NAME%:%ROBOT_SECRET%'))"') do set AUTH_BASE64=%%i

:: Create the docker config JSON content with proper auth field
set DOCKER_CONFIG={"auths":{"%HARBOR_SVC%":{"username":"%ROBOT_FULL_NAME%","password":"%ROBOT_SECRET%","auth":"%AUTH_BASE64%"}}}

echo [6/7] Creating secret in '%K8S_NAMESPACE%' namespace...
kubectl create namespace %K8S_NAMESPACE% 2>nul

kubectl delete secret %K8S_SECRET_NAME% -n %K8S_NAMESPACE% --ignore-not-found >nul 2>&1

kubectl create secret generic %K8S_SECRET_NAME% ^
  --from-literal=config.json="%DOCKER_CONFIG%" ^
  -n %K8S_NAMESPACE%

if errorlevel 1 (
    echo [ERROR] Failed to create Kubernetes secret in %K8S_NAMESPACE%.
    exit /b 1
)
echo [6/7] Secret '%K8S_SECRET_NAME%' created in namespace '%K8S_NAMESPACE%'.

echo [6/7] Creating secret in '%INSTANCE_NAMESPACE%' namespace...
kubectl create namespace %INSTANCE_NAMESPACE% 2>nul

kubectl delete secret %K8S_SECRET_NAME% -n %INSTANCE_NAMESPACE% --ignore-not-found >nul 2>&1

kubectl create secret generic %K8S_SECRET_NAME% ^
  --from-literal=config.json="%DOCKER_CONFIG%" ^
  -n %INSTANCE_NAMESPACE%

if errorlevel 1 (
    echo [ERROR] Failed to create Kubernetes secret in %INSTANCE_NAMESPACE%.
    exit /b 1
)
echo [6/7] Secret '%K8S_SECRET_NAME%' created in namespace '%INSTANCE_NAMESPACE%'.
echo.

:: ============================================================
:: [7/7] Save credentials to file
:: ============================================================
echo [7/7] Saving credentials to file...

for /f "delims=" %%i in ('powershell -NoProfile -Command "Get-Date -Format \"yyyy-MM-dd HH:mm:ss\""') do set TIMESTAMP=%%i

(
    echo Harbor Robot Account for Kaniko
    echo ============================================================
    echo Robot Name   : %ROBOT_FULL_NAME%
    echo Robot Secret : %ROBOT_SECRET%
    echo Harbor URL   : %HARBOR_SVC%
    echo Project      : %PROJECT_NAME%
    echo Created      : %TIMESTAMP%
    echo ============================================================
    echo.
    echo Kaniko --destination example:
    echo   harbor.harbor.svc.cluster.local/%PROJECT_NAME%/myimage:latest
    echo.
    echo Bash exports for workspace pods:
    echo   export HARBOR_URL=harbor.harbor.svc.cluster.local
    echo   export HARBOR_USER='%ROBOT_FULL_NAME%'
    echo   export HARBOR_PASS='%ROBOT_SECRET%'
) > "%CREDENTIALS_FILE%"

echo [7/7] Credentials saved to: %CREDENTIALS_FILE%
echo.

:: ============================================================
:: Final Status
:: ============================================================
echo ============================================================
echo   Deployment Complete - Status
echo ============================================================
echo.
echo --- Harbor Pods ^(%HARBOR_NAMESPACE%^) ---
kubectl get pods -n %HARBOR_NAMESPACE%
echo.
echo --- Harbor Service ---
kubectl get svc harbor -n %HARBOR_NAMESPACE%
echo.
echo --- K8s Secret ^(%K8S_NAMESPACE%^) ---
kubectl get secret %K8S_SECRET_NAME% -n %K8S_NAMESPACE%
echo.
echo --- K8s Secret ^(%INSTANCE_NAMESPACE%^) ---
kubectl get secret %K8S_SECRET_NAME% -n %INSTANCE_NAMESPACE%
echo.
echo ============================================================
echo   Summary
echo ============================================================
echo  Robot Name   : %ROBOT_FULL_NAME%
echo  Robot Secret : %ROBOT_SECRET%
echo  Harbor URL   : %HARBOR_SVC%
echo  Project      : %PROJECT_NAME%
echo  K8s Secret   : %K8S_SECRET_NAME% ^(in %K8S_NAMESPACE%^ and %INSTANCE_NAMESPACE%^)
echo  Saved to     : %CREDENTIALS_FILE%
echo ============================================================
echo.
echo  Bash exports for workspace pods:
echo    export HARBOR_URL=harbor.harbor.svc.cluster.local
echo    export HARBOR_USER='%ROBOT_FULL_NAME%'
echo    export HARBOR_PASS='%ROBOT_SECRET%'
echo.

endlocal
