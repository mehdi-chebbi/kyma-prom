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
			// Init containers: 1) Git clone/setup, 2) Install extensions
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
			},
			// Main code-server container
			Containers: []corev1.Container{
				{
					Name:  "code-server",
					Image: c.config.CodeServerImage,
					Args: []string{
						"--bind-addr", "0.0.0.0:8080",
						"--auth", "none", // Auth handled by Istio/Keycloak
						"--disable-telemetry",
						"--extensions-dir", "/home/coder/workspace/.userdata/.vscode-extensions",
						"--user-data-dir", "/home/coder/workspace/.userdata/.vscode-server",
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
			},
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
