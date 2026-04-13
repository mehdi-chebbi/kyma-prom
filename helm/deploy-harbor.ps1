# ============================================================
#  deploy-harbor.ps1
#  Deploys Harbor (ClusterIP) and automates full setup
#  Harbor API calls are made via a temp pod inside the cluster
# ============================================================

$ErrorActionPreference = "Stop"

$HARBOR_NAMESPACE    = "harbor"
$HARBOR_ADMIN_PASS   = "Harbor12345"
$HARBOR_SVC          = "harbor.harbor.svc.cluster.local"
$HARBOR_API          = "http://$HARBOR_SVC/api/v2.0"
$PROJECT_NAME        = "devplatform"
$ROBOT_NAME          = "kaniko-pusher"
$K8S_SECRET_NAME     = "harbor-credentials"
$K8S_NAMESPACE       = "dev-platform"
$INSTANCE_NAMESPACE  = "codeserver-instances"
$VALUES_FILE         = Join-Path $PSScriptRoot "harbor-values.yaml"
$CREDENTIALS_FILE    = Join-Path $PSScriptRoot "harbor-robot-credentials.txt"
$TMP_POD             = "harbor-setup-tmp"

Write-Host ""
Write-Host "============================================================"
Write-Host "  Harbor Registry Deployment"
Write-Host "============================================================"
Write-Host "  Values file : $VALUES_FILE"
Write-Host "  Namespace   : $HARBOR_NAMESPACE"
Write-Host "  Project     : $PROJECT_NAME"
Write-Host "  K8s secret  : $K8S_SECRET_NAME in $K8S_NAMESPACE"
Write-Host "============================================================"
Write-Host ""

# Verify values file exists
if (-not (Test-Path $VALUES_FILE)) {
    Write-Error "[ERROR] harbor-values.yaml not found at: $VALUES_FILE"
    exit 1
}

# ============================================================
# Helper: run kubectl exec curl command inside the temp pod
# ============================================================
function Invoke-HarborAPI {
    param(
        [string]$Method = "GET",
        [string]$Path,
        [string]$Body = "",
        [hashtable]$ExtraHeaders = @{}
    )

    $args = @(
        "exec", $TMP_POD,
        "-n", $HARBOR_NAMESPACE,
        "--",
        "curl", "-s",
        "-u", "admin:$HARBOR_ADMIN_PASS",
        "-X", $Method,
        "$HARBOR_API$Path"
    )

    foreach ($key in $ExtraHeaders.Keys) {
        $args += "-H"
        $args += "$key`: $($ExtraHeaders[$key])"
    }

    if ($Body -ne "") {
        $args += "-H"
        $args += "Content-Type: application/json"
        $args += "-d"
        $args += $Body
    }

    $result = & kubectl @args
    return $result
}

# ============================================================
# [1/7] Add Harbor Helm repo and update
# ============================================================
Write-Host "[1/7] Adding Harbor Helm repository..."
helm repo add harbor https://helm.goharbor.io 2>$null
Write-Host "[1/7] Updating Helm repositories..."
helm repo update
if ($LASTEXITCODE -ne 0) { Write-Error "[ERROR] Failed to update Helm repos."; exit 1 }
Write-Host "[1/7] Done."
Write-Host ""

# ============================================================
# [2/7] Create namespace and deploy Harbor via Helm
# ============================================================
Write-Host "[2/7] Creating namespace '$HARBOR_NAMESPACE'..."
kubectl create namespace $HARBOR_NAMESPACE 2>$null

Write-Host "[2/7] Deploying Harbor via Helm with harbor-values.yaml..."
helm install harbor harbor/harbor `
    --namespace $HARBOR_NAMESPACE `
    --values $VALUES_FILE

if ($LASTEXITCODE -ne 0) {
    Write-Host "[2/7] Helm install returned non-zero - attempting upgrade..."
    helm upgrade harbor harbor/harbor `
        --namespace $HARBOR_NAMESPACE `
        --values $VALUES_FILE
    if ($LASTEXITCODE -ne 0) {
        Write-Error "[ERROR] Both install and upgrade failed."
        helm status harbor -n $HARBOR_NAMESPACE
        exit 1
    }
}
Write-Host "[2/7] Done."
Write-Host ""

# ============================================================
# [3/7] Wait for Harbor to be ready
# ============================================================
Write-Host "[3/7] Waiting for harbor-core deployment (timeout: 300s)..."
kubectl rollout status deployment/harbor-core -n $HARBOR_NAMESPACE --timeout=300s
if ($LASTEXITCODE -ne 0) {
    Write-Error "[ERROR] harbor-core not ready."
    kubectl get pods -n $HARBOR_NAMESPACE
    exit 1
}

Write-Host "[3/7] Waiting for harbor-registry deployment (timeout: 300s)..."
kubectl rollout status deployment/harbor-registry -n $HARBOR_NAMESPACE --timeout=300s
if ($LASTEXITCODE -ne 0) {
    Write-Error "[ERROR] harbor-registry not ready."
    kubectl get pods -n $HARBOR_NAMESPACE
    exit 1
}
Write-Host "[3/7] Harbor is ready."
Write-Host ""

# ============================================================
# [4/7] Launch temp pod and create devplatform project
# ============================================================
Write-Host "[4/7] Launching temporary curl pod for API calls (Harbor is ClusterIP)..."

# Clean up any leftover temp pod
kubectl delete pod $TMP_POD -n $HARBOR_NAMESPACE --ignore-not-found 2>$null
Start-Sleep -Seconds 3

kubectl run $TMP_POD `
    --image=curlimages/curl:latest `
    --restart=Never `
    --namespace=$HARBOR_NAMESPACE `
    --command -- sleep 300

Write-Host "[4/7] Waiting for temp pod to be ready..."
kubectl wait pod/$TMP_POD `
    --for=condition=Ready `
    --namespace=$HARBOR_NAMESPACE `
    --timeout=60s

if ($LASTEXITCODE -ne 0) {
    Write-Error "[ERROR] Temp pod failed to start."
    kubectl describe pod $TMP_POD -n $HARBOR_NAMESPACE
    exit 1
}

Write-Host "[4/7] Creating project '$PROJECT_NAME' via Harbor API..."
$projBody = '{"project_name":"' + $PROJECT_NAME + '","public":false}'
$projStatusRaw = & kubectl exec $TMP_POD -n $HARBOR_NAMESPACE -- `
    curl -s -o /dev/null -w "%{http_code}" `
    -u "admin:$HARBOR_ADMIN_PASS" `
    -X POST "$HARBOR_API/projects" `
    -H "Content-Type: application/json" `
    -d $projBody

$projStatus = $projStatusRaw.Trim()
Write-Host "[4/7] API response: HTTP $projStatus"

if ($projStatus -eq "201") {
    Write-Host "[4/7] Project '$PROJECT_NAME' created successfully."
} elseif ($projStatus -eq "409") {
    Write-Host "[4/7] Project '$PROJECT_NAME' already exists - continuing."
} else {
    Write-Warning "[WARN] Unexpected response $projStatus from project API - check Harbor logs."
}
Write-Host ""

# ============================================================
# [5/7] Create robot account, capture name and secret
# ============================================================
Write-Host "[5/7] Getting project ID for '$PROJECT_NAME'..."

$projectJsonRaw = & kubectl exec $TMP_POD -n $HARBOR_NAMESPACE -- `
    curl -s `
    -u "admin:$HARBOR_ADMIN_PASS" `
    "$HARBOR_API/projects/$PROJECT_NAME" `
    -H "X-Is-Resource-Name: true"

try {
    $projectObj = $projectJsonRaw | ConvertFrom-Json
    $PROJECT_ID = $projectObj.project_id
} catch {
    Write-Error "[ERROR] Could not parse project JSON: $projectJsonRaw"
    kubectl delete pod $TMP_POD -n $HARBOR_NAMESPACE --ignore-not-found 2>$null
    exit 1
}

if (-not $PROJECT_ID) {
    Write-Error "[ERROR] Could not get project ID."
    kubectl delete pod $TMP_POD -n $HARBOR_NAMESPACE --ignore-not-found 2>$null
    exit 1
}
Write-Host "[5/7] Project ID: $PROJECT_ID"

Write-Host "[5/7] Checking if robot '$ROBOT_NAME' already exists..."
$robotsListRaw = & kubectl exec $TMP_POD -n $HARBOR_NAMESPACE -- `
    curl -s `
    -u "admin:$HARBOR_ADMIN_PASS" `
    "$HARBOR_API/robots?q=Level=project,ProjectID=$PROJECT_ID"

try {
    $robotsList = $robotsListRaw | ConvertFrom-Json
    $existingRobot = $robotsList | Where-Object { $_.name -like "*$ROBOT_NAME" } | Select-Object -First 1
    $EXISTING_ROBOT_ID = $existingRobot.id
} catch {
    $EXISTING_ROBOT_ID = $null
}

if ($EXISTING_ROBOT_ID) {
    Write-Host "[5/7] Robot already exists (ID: $EXISTING_ROBOT_ID), deleting it..."
    & kubectl exec $TMP_POD -n $HARBOR_NAMESPACE -- `
        curl -s `
        -u "admin:$HARBOR_ADMIN_PASS" `
        -X DELETE "$HARBOR_API/robots/$EXISTING_ROBOT_ID" | Out-Null
    Write-Host "[5/7] Existing robot deleted."
}

Write-Host "[5/7] Creating robot account '$ROBOT_NAME'..."
$robotBody = '{"name":"' + $ROBOT_NAME + '","description":"Robot account for Kaniko image builds","duration":-1,"level":"project","permissions":[{"kind":"project","namespace":"' + $PROJECT_NAME + '","access":[{"resource":"repository","action":"push"},{"resource":"repository","action":"pull"},{"resource":"artifact","action":"read"}]}]}'

$robotJsonRaw = & kubectl exec $TMP_POD -n $HARBOR_NAMESPACE -- `
    curl -s `
    -u "admin:$HARBOR_ADMIN_PASS" `
    -X POST "$HARBOR_API/robots" `
    -H "Content-Type: application/json" `
    -d $robotBody

Write-Host "[5/7] Raw robot API response:"
Write-Host $robotJsonRaw
Write-Host ""

try {
    $robotObj = $robotJsonRaw | ConvertFrom-Json
    $ROBOT_FULL_NAME = $robotObj.name
    $ROBOT_SECRET    = $robotObj.secret
} catch {
    Write-Error "[ERROR] Could not parse robot API response."
    kubectl delete pod $TMP_POD -n $HARBOR_NAMESPACE --ignore-not-found 2>$null
    exit 1
}

if (-not $ROBOT_FULL_NAME) {
    Write-Error "[ERROR] Could not parse robot name from API response."
    kubectl delete pod $TMP_POD -n $HARBOR_NAMESPACE --ignore-not-found 2>$null
    exit 1
}
if (-not $ROBOT_SECRET) {
    Write-Error "[ERROR] Could not parse robot secret from API response."
    kubectl delete pod $TMP_POD -n $HARBOR_NAMESPACE --ignore-not-found 2>$null
    exit 1
}

Write-Host "[5/7] Robot account created:"
Write-Host "       Name   : $ROBOT_FULL_NAME"
Write-Host "       Secret : $ROBOT_SECRET"
Write-Host ""

Write-Host "[5/7] Cleaning up temp pod..."
kubectl delete pod $TMP_POD -n $HARBOR_NAMESPACE --ignore-not-found 2>$null

# ============================================================
# [6/7] Create Kubernetes docker-registry secret
# ============================================================
Write-Host "[6/7] Building docker config JSON..."
$authPlain  = "${ROBOT_FULL_NAME}:${ROBOT_SECRET}"
$authBase64 = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($authPlain))
$dockerConfig = '{"auths":{"' + $HARBOR_SVC + '":{"username":"' + $ROBOT_FULL_NAME + '","password":"' + $ROBOT_SECRET + '","auth":"' + $authBase64 + '"}}}'

foreach ($ns in @($K8S_NAMESPACE, $INSTANCE_NAMESPACE)) {
    Write-Host "[6/7] Creating secret in '$ns' namespace..."
    kubectl create namespace $ns 2>$null

    # Add Helm ownership labels so helm install can adopt pre-created namespaces
    kubectl label namespace $ns app.kubernetes.io/managed-by=Helm --overwrite
    kubectl annotate namespace $ns meta.helm.sh/release-name=devplatform meta.helm.sh/release-namespace=default --overwrite

    kubectl delete secret $K8S_SECRET_NAME -n $ns --ignore-not-found 2>$null

    kubectl create secret generic $K8S_SECRET_NAME `
        --from-literal="config.json=$dockerConfig" `
        -n $ns

    if ($LASTEXITCODE -ne 0) {
        Write-Error "[ERROR] Failed to create Kubernetes secret in $ns."
        exit 1
    }
    Write-Host "[6/7] Secret '$K8S_SECRET_NAME' created in namespace '$ns'."
}
Write-Host ""

# ============================================================
# [7/7] Save credentials to file
# ============================================================
Write-Host "[7/7] Saving credentials to file..."
$timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"

@"
Harbor Robot Account for Kaniko
============================================================
Robot Name   : $ROBOT_FULL_NAME
Robot Secret : $ROBOT_SECRET
Harbor URL   : $HARBOR_SVC
Project      : $PROJECT_NAME
Created      : $timestamp
============================================================

Kaniko --destination example:
  harbor.harbor.svc.cluster.local/$PROJECT_NAME/myimage:latest

Bash exports for workspace pods:
  export HARBOR_URL=harbor.harbor.svc.cluster.local
  export HARBOR_USER='$ROBOT_FULL_NAME'
  export HARBOR_PASS='$ROBOT_SECRET'
"@ | Set-Content -Path $CREDENTIALS_FILE -Encoding UTF8

Write-Host "[7/7] Credentials saved to: $CREDENTIALS_FILE"
Write-Host ""

# ============================================================
# Final Status
# ============================================================
Write-Host "============================================================"
Write-Host "  Deployment Complete - Status"
Write-Host "============================================================"
Write-Host ""
Write-Host "--- Harbor Pods ($HARBOR_NAMESPACE) ---"
kubectl get pods -n $HARBOR_NAMESPACE
Write-Host ""
Write-Host "--- Harbor Service ---"
kubectl get svc harbor -n $HARBOR_NAMESPACE
Write-Host ""
Write-Host "--- K8s Secret ($K8S_NAMESPACE) ---"
kubectl get secret $K8S_SECRET_NAME -n $K8S_NAMESPACE
Write-Host ""
Write-Host "--- K8s Secret ($INSTANCE_NAMESPACE) ---"
kubectl get secret $K8S_SECRET_NAME -n $INSTANCE_NAMESPACE
Write-Host ""
Write-Host "============================================================"
Write-Host "  Summary"
Write-Host "============================================================"
Write-Host " Robot Name   : $ROBOT_FULL_NAME"
Write-Host " Robot Secret : $ROBOT_SECRET"
Write-Host " Harbor URL   : $HARBOR_SVC"
Write-Host " Project      : $PROJECT_NAME"
Write-Host " K8s Secret   : $K8S_SECRET_NAME (in $K8S_NAMESPACE and $INSTANCE_NAMESPACE)"
Write-Host " Saved to     : $CREDENTIALS_FILE"
Write-Host "============================================================"
Write-Host ""
Write-Host " Bash exports for workspace pods:"
Write-Host "   export HARBOR_URL=harbor.harbor.svc.cluster.local"
Write-Host "   export HARBOR_USER='$ROBOT_FULL_NAME'"
Write-Host "   export HARBOR_PASS='$ROBOT_SECRET'"
Write-Host ""
