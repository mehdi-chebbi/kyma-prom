package kubernetes

import (
        "bytes"
        "context"
        "fmt"
        "io"
        "time"

        "github.com/devplatform/codeserver-service/internal/models"
        corev1 "k8s.io/api/core/v1"
        "k8s.io/apimachinery/pkg/api/errors"
        "k8s.io/apimachinery/pkg/api/resource"
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
        "k8s.io/apimachinery/pkg/util/intstr"
        "k8s.io/apimachinery/pkg/util/wait"
)

// CreateCodeServerPod creates a new code-server pod for a user
func (c *Client) CreateCodeServerPod(ctx context.Context, userID, repoURL, repoName, repoOwner, branch string) (*corev1.Pod, error) {
        podName := c.config.GetPodName(userID)
        pvcName := c.config.GetPVCName(userID)

        // Check if pod already exists
        existing, err := c.clientset.CoreV1().Pods(c.config.Namespace).Get(ctx, podName, metav1.GetOptions{})
        if err == nil {
                c.logger.WithField("pod", podName).Debug("Pod already exists")
                return existing, nil
        }

        if !errors.IsNotFound(err) {
                return nil, fmt.Errorf("failed to get pod: %w", err)
        }

        // Build and create pod
        pod := c.buildCodeServerPod(userID, repoURL, repoName, repoOwner, pvcName, branch)
        created, err := c.clientset.CoreV1().Pods(c.config.Namespace).Create(ctx, pod, metav1.CreateOptions{})
        if err != nil {
                return nil, fmt.Errorf("failed to create pod: %w", err)
        }

        c.logger.WithFields(map[string]interface{}{
                "pod":      podName,
                "user":     userID,
                "repo":     repoName,
                "repoUrl":  repoURL,
        }).Info("Created code-server pod")

        return created, nil
}

// GetCodeServerPod gets the current pod for a user
func (c *Client) GetCodeServerPod(ctx context.Context, userID string) (*corev1.Pod, error) {
        podName := c.config.GetPodName(userID)
        return c.clientset.CoreV1().Pods(c.config.Namespace).Get(ctx, podName, metav1.GetOptions{})
}

// DeleteCodeServerPod deletes a user's code-server pod
func (c *Client) DeleteCodeServerPod(ctx context.Context, userID string) error {
        podName := c.config.GetPodName(userID)

        err := c.clientset.CoreV1().Pods(c.config.Namespace).Delete(ctx, podName, metav1.DeleteOptions{})
        if err != nil {
                if errors.IsNotFound(err) {
                        return nil
                }
                return fmt.Errorf("failed to delete pod: %w", err)
        }

        c.logger.WithFields(map[string]interface{}{
                "pod":  podName,
                "user": userID,
        }).Info("Deleted code-server pod")

        return nil
}

// GetPodStatus returns the current status of a pod
func (c *Client) GetPodStatus(ctx context.Context, userID string) (models.InstanceStatus, string, error) {
        pod, err := c.GetCodeServerPod(ctx, userID)
        if err != nil {
                if errors.IsNotFound(err) {
                        return models.StatusStopped, "", nil
                }
                return models.StatusError, "", err
        }

        return c.podPhaseToStatus(pod), pod.Status.Message, nil
}

// WaitForPodReady waits for pod to be in Running state
func (c *Client) WaitForPodReady(ctx context.Context, userID string, timeout time.Duration) error {
        podName := c.config.GetPodName(userID)

        return wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
                pod, err := c.clientset.CoreV1().Pods(c.config.Namespace).Get(ctx, podName, metav1.GetOptions{})
                if err != nil {
                        return false, err
                }

                // Check if pod is running and ready
                if pod.Status.Phase == corev1.PodRunning {
                        for _, cond := range pod.Status.Conditions {
                                if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
                                        return true, nil
                                }
                        }
                }

                // Check for failed state
                if pod.Status.Phase == corev1.PodFailed {
                        return false, fmt.Errorf("pod failed: %s", pod.Status.Message)
                }

                return false, nil
        })
}

// GetPodLogs returns logs from a user's pod
func (c *Client) GetPodLogs(ctx context.Context, userID string, lines int64) (string, error) {
        podName := c.config.GetPodName(userID)

        req := c.clientset.CoreV1().Pods(c.config.Namespace).GetLogs(podName, &corev1.PodLogOptions{
                Container: "code-server",
                TailLines: &lines,
        })

        stream, err := req.Stream(ctx)
        if err != nil {
                return "", fmt.Errorf("failed to get logs: %w", err)
        }
        defer stream.Close()

        buf := new(bytes.Buffer)
        _, err = io.Copy(buf, stream)
        if err != nil {
                return "", fmt.Errorf("failed to read logs: %w", err)
        }

        return buf.String(), nil
}

// buildCodeServerPod creates a pod specification for code-server
func (c *Client) buildCodeServerPod(userID, repoURL, repoName, repoOwner, pvcName, branch string) *corev1.Pod {
        podName := c.config.GetPodName(userID)
        sanitizedUser := sanitizeUserID(userID)

        // Git clone command with authentication token embedded in URL
        // Also sets up git credentials for push and persistent shell history
        gitCloneScript := `
set -e
WORKSPACE_DIR="/home/coder/workspace"
REPO_DIR="${WORKSPACE_DIR}/${REPO_NAME}"
USER_DATA_DIR="${WORKSPACE_DIR}/.userdata"

echo "=== Starting workspace setup ==="

# Create persistent user data directory in PVC
mkdir -p "${USER_DATA_DIR}"
mkdir -p "${USER_DATA_DIR}/.git-credentials"
mkdir -p "${USER_DATA_DIR}/.bash_history_dir"

echo "=== Configuring git ==="
# Create persistent gitconfig
GITCONFIG_FILE="${USER_DATA_DIR}/.gitconfig"

cat > "${GITCONFIG_FILE}" << GITCFG
[user]
    email = ${USER_EMAIL:-user@devplatform.local}
    name = ${USER_ID:-coder}
[init]
    defaultBranch = main
[safe]
    directory = ${REPO_DIR}
[credential]
    helper = store --file=${USER_DATA_DIR}/.git-credentials/credentials
[pull]
    rebase = false
[push]
    default = current
[core]
    editor = code --wait
GITCFG

export GIT_CONFIG_GLOBAL="${GITCONFIG_FILE}"
echo "Git config created at ${GITCONFIG_FILE}"

# If GIT_REPO_URL contains credentials, extract and store them
if echo "${GIT_REPO_URL}" | grep -q "@"; then
    # URL format: https://token:TOKEN@host/owner/repo.git
    CRED_HOST=$(echo "${GIT_REPO_URL}" | sed -E 's|https?://[^@]+@([^/]+)/.*|\1|')
    CRED_PROTO=$(echo "${GIT_REPO_URL}" | sed -E 's|(https?)://.*|\1|')
    CRED_USER=$(echo "${GIT_REPO_URL}" | sed -E 's|https?://([^:]+):.*|\1|')
    CRED_PASS=$(echo "${GIT_REPO_URL}" | sed -E 's|https?://[^:]+:([^@]+)@.*|\1|')

    # Store credentials
    echo "${CRED_PROTO}://${CRED_USER}:${CRED_PASS}@${CRED_HOST}" > "${USER_DATA_DIR}/.git-credentials/credentials"
    chmod 600 "${USER_DATA_DIR}/.git-credentials/credentials"
    echo "Git credentials configured for ${CRED_HOST}"
fi

echo "=== Setting up shell history persistence ==="
# Create persistent bash history file
touch "${USER_DATA_DIR}/.bash_history_dir/.bash_history"

# Create .bashrc that uses persistent history
cat > "${USER_DATA_DIR}/.bashrc" << 'BASHRC'
# Persistent bash history
export HISTFILE="/home/coder/workspace/.userdata/.bash_history_dir/.bash_history"
export HISTSIZE=10000
export HISTFILESIZE=20000
export HISTCONTROL=ignoredups:erasedups
shopt -s histappend

# Save history after each command
PROMPT_COMMAND="history -a; history -c; history -r; ${PROMPT_COMMAND}"

# Useful aliases
alias ll='ls -la'
alias gs='git status'
alias gp='git push'
alias gl='git pull'
alias gc='git commit'
alias gd='git diff'
alias build='/home/coder/workspace/.userdata/bin/build'

# Show git branch in prompt
parse_git_branch() {
    git branch 2>/dev/null | sed -e '/^[^*]/d' -e 's/* \(.*\)/ (\1)/'
}
export PS1='\[\033[01;32m\]\u@code-server\[\033[00m\]:\[\033[01;34m\]\w\[\033[33m\]$(parse_git_branch)\[\033[00m\]\$ '

echo "Welcome to Code Server - ${REPO_NAME}"
BASHRC

echo "=== Cloning/updating repository ==="
# Clone or pull
if [ ! -d "${REPO_DIR}/.git" ]; then
    echo "Cloning repository..."
    if [ -n "${BRANCH}" ]; then
        echo "Cloning branch: ${BRANCH}"
        git clone --branch "${BRANCH}" "${GIT_REPO_URL}" "${REPO_DIR}"
    else
        git clone "${GIT_REPO_URL}" "${REPO_DIR}"
    fi
    echo "Repository cloned successfully"

    # Set remote URL without credentials for security (credentials in store)
    cd "${REPO_DIR}"
    CLEAN_URL=$(echo "${GIT_REPO_URL}" | sed -E 's|(https?://)([^@]+@)?(.*)|\1\3|')
    git remote set-url origin "${CLEAN_URL}" || true
else
    echo "Repository exists, pulling latest changes..."
    cd "${REPO_DIR}"
    git fetch --all
    if [ -n "${BRANCH}" ]; then
        echo "Checking out branch: ${BRANCH}"
        git checkout "${BRANCH}" 2>/dev/null || git checkout -b "${BRANCH}" "origin/${BRANCH}" 2>/dev/null || echo "Branch checkout failed, staying on current branch"
    fi
    git pull || echo "Pull failed, continuing with existing code"
    echo "Repository updated"
fi

echo "=== Setting permissions ==="
# Set ownership
chown -R 1000:1000 "${WORKSPACE_DIR}" || true
chmod 700 "${USER_DATA_DIR}/.git-credentials" || true

echo "=== Workspace setup completed ==="
`

        // VS Code extensions to pre-install (stored in PVC for persistence)
        extensionsScript := `
set -e
echo "=== Installing VS Code extensions ==="

EXTENSIONS_DIR="/home/coder/workspace/.userdata/.vscode-extensions"
USER_DATA_DIR="/home/coder/workspace/.userdata/.vscode-server"

mkdir -p "${EXTENSIONS_DIR}"
mkdir -p "${USER_DATA_DIR}"

# Common extensions to pre-install
EXTENSIONS="
ms-python.python
golang.go
esbenp.prettier-vscode
dbaeumer.vscode-eslint
eamodio.gitlens
formulahendry.auto-rename-tag
bradlc.vscode-tailwindcss
PKief.material-icon-theme
redhat.vscode-yaml
ms-azuretools.vscode-docker
"

for ext in $EXTENSIONS; do
    # Check if extension already installed
    if [ -d "${EXTENSIONS_DIR}/${ext%.*}"* ] 2>/dev/null; then
        echo "Extension $ext already installed, skipping..."
    else
        echo "Installing extension: $ext"
        code-server --extensions-dir "${EXTENSIONS_DIR}" --install-extension "$ext" || echo "Failed to install $ext, continuing..."
    fi
done

echo "=== Extensions installation completed ==="
`

        // Setup build tools (kaniko build script + jq binary)
        buildToolsScript := `
set -e
echo "=== Setting up build tools ==="

TOOLS_DIR="/home/coder/workspace/.userdata/bin"
mkdir -p "${TOOLS_DIR}"

# Download static jq binary (works on any Linux, no deps)
if [ ! -f "${TOOLS_DIR}/jq" ]; then
    echo "Downloading jq..."
    wget -q -O "${TOOLS_DIR}/jq" https://github.com/jqlang/jq/releases/download/jq-1.7.1/jq-linux-amd64 || \
    wget -q -O "${TOOLS_DIR}/jq" https://github.com/jqlang/jq/releases/download/jq-1.6/jq-linux64 || \
    echo "WARNING: Failed to download jq, build script will use fallback parsing"
    chmod +x "${TOOLS_DIR}/jq" 2>/dev/null || true
fi

# Write the build helper script
cat > "${TOOLS_DIR}/build" << 'BUILDSCRIPT'
#!/bin/bash
# build - Build and push Docker image using Kaniko
# Usage: build --dockerfile <path> [--context <path>]

set -e

# Parse arguments
DOCKERFILE_ARG=""
CONTEXT_ARG="."
while [[ $# -gt 0 ]]; do
    case $1 in
        --dockerfile|-f)
            DOCKERFILE_ARG="$2"
            shift 2
            ;;
        --context|-c)
            CONTEXT_ARG="$2"
            shift 2
            ;;
        --help|-h)
            echo "Usage: build --dockerfile <path> [--context <path>]"
            echo ""
            echo "Options:"
            echo "  --dockerfile, -f  Path to Dockerfile (required)"
            echo "  --context, -c    Build context directory (default: .)"
            echo ""
            echo "Example:"
            echo "  build --dockerfile ./Dockerfile"
            echo "  build --dockerfile ./docker/Dockerfile --context ./app"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Run 'build --help' for usage."
            exit 1
            ;;
    esac
done

if [ -z "$DOCKERFILE_ARG" ]; then
    echo "Error: --dockerfile is required."
    echo "Usage: build --dockerfile <path>"
    exit 1
fi

# Resolve paths relative to current working directory
WORKSPACE_DIR="/home/coder/workspace"
REPO_DIR="${WORKSPACE_DIR}/${REPO_NAME}"
CWD="$(cd "$(dirname "$DOCKERFILE_ARG")" 2>/dev/null && pwd)"

# Resolve full dockerfile path from CWD
if [[ "$DOCKERFILE_ARG" = /* ]]; then
    FULL_DOCKERFILE="$DOCKERFILE_ARG"
else
    FULL_DOCKERFILE="${CWD}/$(basename "$DOCKERFILE_ARG")"
fi

# Resolve context path from CWD (default to dockerfile's directory)
if [[ "$CONTEXT_ARG" = /* ]]; then
    FULL_CONTEXT="$CONTEXT_ARG"
elif [ "$CONTEXT_ARG" = "." ]; then
    FULL_CONTEXT="$CWD"
else
    FULL_CONTEXT="${CWD}/${CONTEXT_ARG}"
fi

# Validate dockerfile exists
if [ ! -f "$FULL_DOCKERFILE" ]; then
    echo "Error: Dockerfile not found at ${FULL_DOCKERFILE}"
    exit 1
fi

# Validate context exists
if [ ! -d "$FULL_CONTEXT" ]; then
    echo "Error: Context directory not found at ${FULL_CONTEXT}"
    exit 1
fi

# Compute dockerfile path relative to context
DOCKERFILE_REL=$(realpath --relative-to="$FULL_CONTEXT" "$FULL_DOCKERFILE" 2>/dev/null || \
    python3 -c "import os; print(os.path.relpath('$FULL_DOCKERFILE','$FULL_CONTEXT'))" 2>/dev/null || \
    echo "Dockerfile")

# Compute context path relative to workspace root for the kaniko pod mount
# PVC is mounted at /workspace in the kaniko pod, so we need the subpath within the workspace
CONTEXT_SUBPATH=$(realpath --relative-to="$WORKSPACE_DIR" "$FULL_CONTEXT" 2>/dev/null || \
    python3 -c "import os; print(os.path.relpath('$FULL_CONTEXT','$WORKSPACE_DIR'))" 2>/dev/null || \
    echo ".")
KANIKO_CONTEXT="/workspace/${CONTEXT_SUBPATH}"

# Image name: harbor.harbor.svc.cluster.local/devplatform/<last-folder-name>:latest
FOLDER_NAME=$(basename "$FULL_CONTEXT")
IMAGE_NAME="harbor.harbor.svc.cluster.local/devplatform/${FOLDER_NAME}:latest"

# Sanitize user ID for Kubernetes resource names (same logic as Go sanitizeUserID)
SANITIZED_USER=$(echo "$USER_ID" | tr '[:upper:]' '[:lower:]' | sed 's/[._@]/-/g' | sed 's/^-//;s/-$//')
JOB_NAME="build-${SANITIZED_USER}-$(date +%s)"
KUBE_NAMESPACE=$(cat /var/run/secrets/kubernetes.io/serviceaccount/namespace)
KUBE_TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
KUBE_API="https://kubernetes.default.svc"
KUBE_CA="/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"

# jq path
JQ_BIN="/home/coder/workspace/.userdata/bin/jq"
if [ ! -x "$JQ_BIN" ]; then
    JQ_BIN=""
fi

echo "========================================"
echo "  Kaniko Build"
echo "========================================"
echo "Image:      ${IMAGE_NAME}"
echo "Dockerfile: ${DOCKERFILE_REL}"
echo "Context:    ${CONTEXT_SUBPATH}"
echo "Job:        ${JOB_NAME}"
echo "========================================"

# Create the Kaniko Job JSON
JOB_JSON=$(cat <<ENDJOB
{
  "apiVersion": "batch/v1",
  "kind": "Job",
  "metadata": {
    "name": "${JOB_NAME}",
    "namespace": "${KUBE_NAMESPACE}"
  },
  "spec": {
    "backoffLimit": 0,
    "ttlSecondsAfterFinished": 300,
    "template": {
      "metadata": {
        "labels": {
          "app": "kaniko-build",
          "user": "${SANITIZED_USER}"
        },
        "annotations": {
          "sidecar.istio.io/inject": "false"
        }
      },
      "spec": {
        "restartPolicy": "Never",
        "containers": [{
          "name": "kaniko",
          "image": "gcr.io/kaniko-project/executor:latest",
          "args": [
            "--dockerfile=${DOCKERFILE_REL}",
            "--context=dir://${KANIKO_CONTEXT}",
            "--destination=${IMAGE_NAME}",
            "--insecure",
            "--cache=false",
            "--verbosity=info"
          ],
          "volumeMounts": [
            {"name": "workspace", "mountPath": "/workspace"},
            {"name": "harbor-config", "mountPath": "/kaniko/.docker", "readOnly": true}
          ]
        }],
        "volumes": [
          {"name": "workspace", "persistentVolumeClaim": {"claimName": "${PVC_NAME}"}},
          {"name": "harbor-config", "secret": {"secretName": "harbor-credentials"}}
        ]
      }
    }
  }
}
ENDJOB
)

echo "Creating build job..."

HTTP_CODE=$(curl -sk -o /tmp/kaniko-job-response.json -w "%{http_code}" \
    -X POST "${KUBE_API}/apis/batch/v1/namespaces/${KUBE_NAMESPACE}/jobs" \
    -H "Authorization: Bearer ${KUBE_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "${JOB_JSON}")

if [ "$HTTP_CODE" -ne 201 ]; then
    echo "Error: Failed to create build job (HTTP ${HTTP_CODE})"
    cat /tmp/kaniko-job-response.json 2>/dev/null || true
    exit 1
fi

echo "Job created: ${JOB_NAME}"

# Wait for the build pod to be created
echo "Waiting for build pod..."
POD_NAME=""
for i in $(seq 1 30); do
    RESPONSE=$(curl -sk -H "Authorization: Bearer ${KUBE_TOKEN}" \
        "${KUBE_API}/api/v1/namespaces/${KUBE_NAMESPACE}/pods?labelSelector=job-name=${JOB_NAME}" 2>/dev/null)
    if [ -n "$JQ_BIN" ]; then
        POD_NAME=$(echo "$RESPONSE" | "$JQ_BIN" -r '.items[0].metadata.name // empty' 2>/dev/null)
    else
        POD_NAME=$(echo "$RESPONSE" | grep -o '"name":"[^"]*"' | head -1 | sed 's/"name":"//;s/"//')
    fi
    if [ -n "$POD_NAME" ]; then
        break
    fi
    sleep 2
done

if [ -z "$POD_NAME" ]; then
    echo "Error: Build pod was not created within 60s"
    curl -sk -X DELETE "${KUBE_API}/apis/batch/v1/namespaces/${KUBE_NAMESPACE}/jobs/${JOB_NAME}" \
        -H "Authorization: Bearer ${KUBE_TOKEN}" > /dev/null 2>&1 || true
    exit 1
fi

echo "Build pod: ${POD_NAME}"

# Wait for the pod to reach Running phase (image pull, volume mounts, etc.)
echo "Waiting for build pod to start..."
POD_RUNNING="false"
for i in $(seq 1 60); do
    POD_RESPONSE=$(curl -sk -H "Authorization: Bearer ${KUBE_TOKEN}" \
        "${KUBE_API}/api/v1/namespaces/${KUBE_NAMESPACE}/pods/${POD_NAME}" 2>/dev/null)
    if [ -n "$JQ_BIN" ]; then
        POD_PHASE=$(echo "$POD_RESPONSE" | "$JQ_BIN" -r '.status.phase // ""' 2>/dev/null)
    else
        POD_PHASE=$(echo "$POD_RESPONSE" | grep -o '"phase":"[^"]*"' | head -1 | sed 's/"phase":"//;s/"//')
    fi
    if [ "$POD_PHASE" = "Running" ]; then
        POD_RUNNING="true"
        break
    fi
    if [ "$POD_PHASE" = "Failed" ]; then
        echo "Error: Build pod failed to start"
        echo "Pod details:"
        echo "$POD_RESPONSE" 2>/dev/null | head -30
        echo ""
        echo "Inspect manually with:"
        echo "  kubectl describe pod ${POD_NAME} -n ${KUBE_NAMESPACE}"
        echo "Job kept for debugging (not auto-deleted)."
        exit 1
    fi
    sleep 3
done

if [ "$POD_RUNNING" = "false" ]; then
    echo "Error: Build pod did not reach Running state within 180s"
    echo "It may still be pulling the kaniko image from gcr.io."
    echo "Check pod status with:"
    echo "  kubectl get pod ${POD_NAME} -n ${KUBE_NAMESPACE}"
    echo "Job kept for debugging (not auto-deleted)."
    exit 1
fi

echo ""
echo "=== Build Logs ==="
echo "----------------------------------------"

# Tail logs (follow until container exits)
curl -sk -N --max-time 600 \
    -H "Authorization: Bearer ${KUBE_TOKEN}" \
    "${KUBE_API}/api/v1/namespaces/${KUBE_NAMESPACE}/pods/${POD_NAME}/log?follow=true&container=kaniko" 2>/dev/null || true

echo ""
echo "----------------------------------------"

# Wait a moment for job status to settle
sleep 3

# Check job result
JOB_RESPONSE=$(curl -sk -H "Authorization: Bearer ${KUBE_TOKEN}" \
    "${KUBE_API}/apis/batch/v1/namespaces/${KUBE_NAMESPACE}/jobs/${JOB_NAME}" 2>/dev/null)

JOB_SUCCEEDED="false"
JOB_FAILED="false"
if [ -n "$JQ_BIN" ]; then
    JOB_SUCCEEDED=$(echo "$JOB_RESPONSE" | "$JQ_BIN" -r '.status.succeeded // 0' 2>/dev/null)
    JOB_FAILED=$(echo "$JOB_RESPONSE" | "$JQ_BIN" -r '.status.failed // 0' 2>/dev/null)
else
    JOB_SUCCEEDED=$(echo "$JOB_RESPONSE" | grep -o '"succeeded":[0-9]*' | head -1 | grep -o '[0-9]*')
    JOB_FAILED=$(echo "$JOB_RESPONSE" | grep -o '"failed":[0-9]*' | head -1 | grep -o '[0-9]*')
fi

if [ "$JOB_SUCCEEDED" != "0" ]; then
    echo ""
    echo "========================================"
    echo "  Build Successful!"
    echo "========================================"
    echo "Image: ${IMAGE_NAME}"
    echo ""
    echo "Pull it with:"
    echo "  docker pull ${IMAGE_NAME}"
    echo "========================================"
    # Cleanup only on success
    echo "Cleaning up build job..."
    curl -sk -X DELETE "${KUBE_API}/apis/batch/v1/namespaces/${KUBE_NAMESPACE}/jobs/${JOB_NAME}" \
        -H "Authorization: Bearer ${KUBE_TOKEN}" > /dev/null 2>&1 || true
    echo "Done."
else
    echo ""
    echo "========================================"
    echo "  Build Failed"
    echo "========================================"
    echo "Check the logs above for errors."
    echo "Common issues:"
    echo "  - Invalid Dockerfile syntax"
    echo "  - Missing files in build context"
    echo "  - Network issues pulling base images"
    echo ""
    echo "Job kept for debugging. Inspect with:"
    echo "  kubectl describe pod ${POD_NAME} -n ${KUBE_NAMESPACE}"
    echo "  kubectl logs ${POD_NAME} -n ${KUBE_NAMESPACE}"
    echo "Clean up manually when done:"
    echo "  kubectl delete job ${JOB_NAME} -n ${KUBE_NAMESPACE}"
    echo "========================================"
    exit 1
fi
BUILDSCRIPT

chmod +x "${TOOLS_DIR}/build"
chown -R 1000:1000 "${TOOLS_DIR}" || true

echo "=== Build tools setup completed ==="
`

        return &corev1.Pod{
                ObjectMeta: metav1.ObjectMeta{
                        Name:      podName,
                        Namespace: c.config.Namespace,
                        Labels: map[string]string{
                                "app":        "code-server",
                                "user":       sanitizedUser,
                                "repo":       sanitizeUserID(repoName),
                                "repo-owner": sanitizeUserID(repoOwner),
                                "managed-by": "codeserver-service",
                        },
                        Annotations: map[string]string{
                                "codeserver.devplatform/user-id":    userID,
                                "codeserver.devplatform/repo-name":  repoName,
                                "codeserver.devplatform/repo-owner": repoOwner,
                                "codeserver.devplatform/repo-url":   repoURL,
                                "codeserver.devplatform/branch":     branch,
                                // Istio sidecar annotations for WebSocket support
                                "sidecar.istio.io/inject":                          "true",
                                "proxy.istio.io/config":                             `{"holdApplicationUntilProxyStarts": true}`,
                                "traffic.sidecar.istio.io/excludeOutboundIPRanges": "",
                        },
                },
                Spec: corev1.PodSpec{
                        // Init containers: 1) Git clone/setup, 2) Install extensions, 3) Setup build tools
                        InitContainers: []corev1.Container{
                                {
                                        Name:    "git-clone",
                                        Image:   "alpine/git:latest",
                                        Command: []string{"/bin/sh", "-c"},
                                        Args:    []string{gitCloneScript},
                                        Env: []corev1.EnvVar{
                                                {Name: "GIT_REPO_URL", Value: repoURL},
                                                {Name: "REPO_NAME", Value: repoName},
                                                {Name: "USER_ID", Value: userID},
                                                {Name: "USER_EMAIL", Value: userID + "@devplatform.local"},
                                                {Name: "BRANCH", Value: branch},
                                        },
                                        VolumeMounts: []corev1.VolumeMount{
                                                {Name: "workspace", MountPath: "/home/coder/workspace"},
                                        },
                                        SecurityContext: &corev1.SecurityContext{
                                                RunAsUser:  int64Ptr(0), // Root for git operations
                                                RunAsGroup: int64Ptr(0),
                                        },
                                },
                                {
                                        Name:    "install-extensions",
                                        Image:   c.config.CodeServerImage,
                                        Command: []string{"/bin/sh", "-c"},
                                        Args:    []string{extensionsScript},
                                        VolumeMounts: []corev1.VolumeMount{
                                                {Name: "workspace", MountPath: "/home/coder/workspace"},
                                                {Name: "coder-config", MountPath: "/home/coder/.config"},
                                                {Name: "coder-local", MountPath: "/home/coder/.local"},
                                        },
                                        SecurityContext: &corev1.SecurityContext{
                                                RunAsUser:  int64Ptr(1000),
                                                RunAsGroup: int64Ptr(1000),
                                        },
                                },
                                {
                                        Name:    "setup-build-tools",
                                        Image:   "alpine:3.19",
                                        Command: []string{"/bin/sh", "-c"},
                                        Args:    []string{buildToolsScript},
                                        Env: []corev1.EnvVar{
                                                {Name: "REPO_NAME", Value: repoName},
                                                {Name: "USER_ID", Value: userID},
                                                {Name: "PVC_NAME", Value: pvcName},
                                        },
                                        VolumeMounts: []corev1.VolumeMount{
                                                {Name: "workspace", MountPath: "/home/coder/workspace"},
                                        },
                                        SecurityContext: &corev1.SecurityContext{
                                                RunAsUser:  int64Ptr(0),
                                                RunAsGroup: int64Ptr(0),
                                        },
                                },
                        },
                        // Main code-server container
                        Containers: []corev1.Container{
                                {
                                        Name:    "code-server",
                                        Image:   c.config.CodeServerImage,
                                        Command: []string{"/bin/sh", "-c"},
                                        Args: []string{
                                                "ln -sf /home/coder/workspace/.userdata/.bashrc /home/coder/.bashrc 2>/dev/null; " +
                                                        "exec /usr/bin/code-server --bind-addr 0.0.0.0:8080 --auth none --disable-telemetry " +
                                                        "--extensions-dir /home/coder/workspace/.userdata/.vscode-extensions " +
                                                        "--user-data-dir /home/coder/workspace/.userdata/.vscode-server " +
                                                        "/home/coder/workspace/" + repoName,
                                        },
                                        TTY:   true, // Enable TTY for terminal
                                        Stdin: true, // Enable stdin for interactive terminal
                                        Ports: []corev1.ContainerPort{
                                                {
                                                        Name:          "http",
                                                        ContainerPort: 8080,
                                                        Protocol:      corev1.ProtocolTCP,
                                                },
                                        },
                                        Env: []corev1.EnvVar{
                                                {Name: "PASSWORD", Value: ""}, // No password, auth via Istio
                                                {Name: "REPO_NAME", Value: repoName},
                                                {Name: "USER_ID", Value: userID},
                                                // Terminal support
                                                {Name: "SHELL", Value: "/bin/bash"},
                                                {Name: "TERM", Value: "xterm-256color"},
                                                {Name: "HOME", Value: "/home/coder"},
                                                {Name: "LANG", Value: "en_US.UTF-8"},
                                                // Persistent bash config - sourced on every bash session
                                                {Name: "BASH_ENV", Value: "/home/coder/workspace/.userdata/.bashrc"},
                                                // Persistent history
                                                {Name: "HISTFILE", Value: "/home/coder/workspace/.userdata/.bash_history_dir/.bash_history"},
                                                // Git credentials
                                                {Name: "GIT_CONFIG_GLOBAL", Value: "/home/coder/workspace/.userdata/.gitconfig"},
                                                // Kaniko build tools
                                                {Name: "PVC_NAME", Value: pvcName},
                                                // Add userdata bin to PATH so 'build' command is found directly
                                                {Name: "PATH", Value: "/home/coder/workspace/.userdata/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
                                        },
                                        VolumeMounts: []corev1.VolumeMount{
                                                {Name: "workspace", MountPath: "/home/coder/workspace"},
                                                {Name: "tmp", MountPath: "/tmp"},
                                                {Name: "coder-config", MountPath: "/home/coder/.config"},
                                                {Name: "coder-local", MountPath: "/home/coder/.local"},
                                        },
                                        Resources: corev1.ResourceRequirements{
                                                Requests: corev1.ResourceList{
                                                        corev1.ResourceCPU:    resource.MustParse(c.config.CodeServerCPU),
                                                        corev1.ResourceMemory: resource.MustParse(c.config.CodeServerMemory),
                                                },
                                                Limits: corev1.ResourceList{
                                                        corev1.ResourceCPU:    resource.MustParse(c.config.CodeServerCPUMax),
                                                        corev1.ResourceMemory: resource.MustParse(c.config.CodeServerMemMax),
                                                },
                                        },
                                        ReadinessProbe: &corev1.Probe{
                                                ProbeHandler: corev1.ProbeHandler{
                                                        HTTPGet: &corev1.HTTPGetAction{
                                                                Path: "/",
                                                                Port: intstr.FromInt(8080),
                                                        },
                                                },
                                                InitialDelaySeconds: 10,
                                                PeriodSeconds:       5,
                                                TimeoutSeconds:      3,
                                                SuccessThreshold:    1,
                                                FailureThreshold:    3,
                                        },
                                        LivenessProbe: &corev1.Probe{
                                                ProbeHandler: corev1.ProbeHandler{
                                                        HTTPGet: &corev1.HTTPGetAction{
                                                                Path: "/",
                                                                Port: intstr.FromInt(8080),
                                                        },
                                                },
                                                InitialDelaySeconds: 30,
                                                PeriodSeconds:       30,
                                                TimeoutSeconds:      5,
                                                SuccessThreshold:    1,
                                                FailureThreshold:    3,
                                        },
                                        SecurityContext: &corev1.SecurityContext{
                                                RunAsUser:                int64Ptr(1000),
                                                RunAsGroup:               int64Ptr(1000),
                                                AllowPrivilegeEscalation: boolPtr(false),
                                        },
                                },
                        },
                        Volumes: []corev1.Volume{
                                {
                                        Name: "workspace",
                                        VolumeSource: corev1.VolumeSource{
                                                PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
                                                        ClaimName: pvcName,
                                                },
                                        },
                                },
                                {
                                        Name: "tmp",
                                        VolumeSource: corev1.VolumeSource{
                                                EmptyDir: &corev1.EmptyDirVolumeSource{},
                                        },
                                },
                                {
                                        Name: "coder-config",
                                        VolumeSource: corev1.VolumeSource{
                                                EmptyDir: &corev1.EmptyDirVolumeSource{},
                                        },
                                },
                                {
                                        Name: "coder-local",
                                        VolumeSource: corev1.VolumeSource{
                                                EmptyDir: &corev1.EmptyDirVolumeSource{},
                                        },
                                },
                                {
                                        Name: "harbor-config",
                                        VolumeSource: corev1.VolumeSource{
                                                Secret: &corev1.SecretVolumeSource{
                                                        SecretName: "harbor-credentials",
                                                },
                                        },
                                },
                        },
                        ServiceAccountName: "workspace-builder",
                        SecurityContext: &corev1.PodSecurityContext{
                                FSGroup: int64Ptr(1000),
                        },
                        RestartPolicy: corev1.RestartPolicyAlways,
                },
        }
}

// podPhaseToStatus converts Kubernetes pod phase to our status
func (c *Client) podPhaseToStatus(pod *corev1.Pod) models.InstanceStatus {
        switch pod.Status.Phase {
        case corev1.PodPending:
                // Check if it's starting (containers creating) or truly pending
                for _, cs := range pod.Status.ContainerStatuses {
                        if cs.State.Waiting != nil {
                                return models.StatusStarting
                        }
                }
                return models.StatusPending
        case corev1.PodRunning:
                // Check if ready
                for _, cond := range pod.Status.Conditions {
                        if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
                                return models.StatusRunning
                        }
                }
                return models.StatusStarting
        case corev1.PodSucceeded:
                return models.StatusStopped
        case corev1.PodFailed:
                return models.StatusError
        default:
                return models.StatusError
        }
}

// PodToInstance converts a Kubernetes pod to a CodeServerInstance
func (c *Client) PodToInstance(pod *corev1.Pod) *models.CodeServerInstance {
        userID := pod.Annotations["codeserver.devplatform/user-id"]
        repoName := pod.Annotations["codeserver.devplatform/repo-name"]
        repoOwner := pod.Annotations["codeserver.devplatform/repo-owner"]

        instance := &models.CodeServerInstance{
                ID:          pod.Name,
                UserID:      userID,
                RepoName:    repoName,
                RepoOwner:   repoOwner,
                URL:         c.config.GetCodeServerURL(userID),
                Status:      c.podPhaseToStatus(pod),
                CreatedAt:   pod.CreationTimestamp.Time,
                PodName:     pod.Name,
                PVCName:     c.config.GetPVCName(userID),
                ServiceName: c.config.GetServiceName(userID),
        }

        if pod.Status.Message != "" {
                instance.ErrorMessage = pod.Status.Message
        }

        return instance
}

// Helper functions
func int64Ptr(i int64) *int64 {
        return &i
}

func boolPtr(b bool) *bool {
        return &b
}
