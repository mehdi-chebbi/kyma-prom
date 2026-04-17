package models

import "time"

// InstanceStatus represents the status of a code-server instance
type InstanceStatus string

const (
	StatusPending  InstanceStatus = "PENDING"
	StatusStarting InstanceStatus = "STARTING"
	StatusRunning  InstanceStatus = "RUNNING"
	StatusStopping InstanceStatus = "STOPPING"
	StatusStopped  InstanceStatus = "STOPPED"
	StatusError    InstanceStatus = "ERROR"
)

// CodeServerInstance represents a user's code-server instance
type CodeServerInstance struct {
	ID             string         `json:"id"`
	UserID         string         `json:"userId"`
	RepoName       string         `json:"repoName"`
	RepoOwner      string         `json:"repoOwner"`
	URL            string         `json:"url"`
	Status         InstanceStatus `json:"status"`
	CreatedAt      time.Time      `json:"createdAt"`
	LastAccessedAt *time.Time     `json:"lastAccessedAt,omitempty"`
	StorageUsed    string         `json:"storageUsed,omitempty"`
	PodName        string         `json:"podName"`
	PVCName        string         `json:"pvcName"`
	ServiceName    string         `json:"serviceName"`
	ErrorMessage   string         `json:"errorMessage,omitempty"`
}

// ProvisionResult is the result of provisioning a code-server instance
type ProvisionResult struct {
	Instance *CodeServerInstance `json:"instance"`
	Message  string              `json:"message"`
	IsNew    bool                `json:"isNew"`
}

// InstanceStats contains aggregate statistics about instances
type InstanceStats struct {
	TotalInstances   int    `json:"totalInstances"`
	RunningInstances int    `json:"runningInstances"`
	StoppedInstances int    `json:"stoppedInstances"`
	PendingInstances int    `json:"pendingInstances"`
	TotalStorageUsed string `json:"totalStorageUsed"`
}

// Repository represents a Gitea repository
type Repository struct {
	ID       int64  `json:"id"`
	Owner    string `json:"owner"`
	Name     string `json:"name"`
	FullName string `json:"fullName"`
	CloneURL string `json:"cloneUrl"`
	SSHURL   string `json:"sshUrl"`
	HTMLURL  string `json:"htmlUrl"`
	Private  bool   `json:"private"`
}

// User represents a user from the auth context
type User struct {
	UID        string `json:"uid"`
	Email      string `json:"email"`
	Department string `json:"department"`
}

// ProvisionInput is the input for provisioning a code-server
type ProvisionInput struct {
	RepoOwner string `json:"repoOwner"`
	RepoName  string `json:"repoName"`
}

// HealthStatus represents the health of the service
type HealthStatus struct {
	Status      string `json:"status"`
	Kubernetes  bool   `json:"kubernetes"`
	GiteaAccess bool   `json:"giteaAccess"`
}
