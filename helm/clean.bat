@echo off
setlocal enabledelayedexpansion

echo ============================================
echo   DevPlatform Cleanup
echo ============================================
echo.
echo This will remove ALL deployed resources.
echo Press Ctrl+C to cancel, or
pause

REM Check prerequisites
where helm >nul 2>&1 || (echo ERROR: helm not found in PATH && exit /b 1)
where nerdctl >nul 2>&1 || (echo ERROR: nerdctl not found in PATH && exit /b 1)
where kubectl >nul 2>&1 || (echo ERROR: kubectl not found in PATH && exit /b 1)

echo.
echo [1/4] Deleting workspace pods and PVCs...
echo.
kubectl delete pods --all -n codeserver-instances --ignore-not-found 2>nul
kubectl delete pvc --all -n codeserver-instances --ignore-not-found 2>nul

echo.
echo [2/4] Uninstalling Helm release (devplatform)...
echo.
helm uninstall devplatform --ignore-not-found --timeout 60s 2>nul

echo.
echo [3/4] Uninstalling Harbor...
echo.
helm uninstall harbor -n harbor --ignore-not-found --timeout 60s 2>nul

echo.
echo [4/4] Deleting namespaces...
echo.
kubectl delete namespace codeserver-instances --ignore-not-found --timeout=60s 2>nul
kubectl delete namespace dev-platform --ignore-not-found --timeout=60s 2>nul
kubectl delete namespace auth-system --ignore-not-found --timeout=60s 2>nul
kubectl delete namespace harbor --ignore-not-found --timeout=60s 2>nul

echo.
echo [5/4] Removing local images...
echo.
for %%I in (ldap-manager gitea-service gitea-sync-controller codeserver-service code-server ldap-init) do (
    echo Removing %%I:latest...
    nerdctl rmi %%I:latest --force 2>nul || echo   (not found, skipping)
)

echo.
echo ============================================
echo   Cleanup Complete!
echo ============================================
echo.
echo You can now run deploy-harbor.ps1 then deploy.bat
echo to start fresh.
echo.

endlocal
