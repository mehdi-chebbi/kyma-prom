# CLAUDE.md - Code Server Provisioning Service

This file provides guidance to Claude Code for generating a production-grade VS Code Server provisioning microservice.

## Project Overview

Build a microservice in Go that provisions on-demand VS Code Server instances (code-server) for users. Each user gets a personalized, persistent development environment with their Gitea repositories pre-cloned. The service manages Kubernetes pods, persistent volumes, and integrates with the existing DevPlatform authentication stack.

## Technology Stack

- **Language**: Go 1.21+
- **API**: GraphQL (github.com/graphql-go/graphql)
- **Container Runtime**: Kubernetes API (k8s.io/client-go)
- **VS Code Server**: codercom/code-server:latest
- **Auth**: JWT (RS256) via Keycloak, validated by Istio
- **Logging**: logrus with JSON formatting
- **Metrics**: Prometheus client
- **Container**: Multi-stage Dockerfile
- **Orchestration**: Kubernetes with Istio ingress
- **Persistence**: PersistentVolumeClaims per user

## Authentication Architecture

### JWT Token Flow (Single Token for All Services)

The same Keycloak JWT token flows through the entire service chain. No service-specific secrets needed between microservices.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Authentication Flow                              │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  1. User authenticates with Keycloak                                     │
│     POST /realms/devplatform/protocol/openid-connect/token               │
│                           ↓                                              │
│  2. Keycloak returns JWT (signed with RS256 private key)                 │
│     - Header: {"alg":"RS256","typ":"JWT","kid":"..."}                   │
│     - Payload: {preferred_username, email, realm_access, ...}           │
│     - Signature: RSA-SHA256(header.payload, privateKey)                  │
│                           ↓                                              │
│  3. User sends request with JWT                                          │
│     Authorization: Bearer eyJhbGciOiJSUzI1Ni...                         │
│                           ↓                                              │
│  4. Istio Gateway validates JWT using JWKS                               │
│     - Fetches public keys from Keycloak JWKS endpoint                   │
│     - Verifies signature matches kid in token header                     │
│     - Injects X-Forwarded-User, X-Forwarded-Email headers               │
│                           ↓                                              │
│  5. CodeServer Service receives validated request                        │
│     - Extracts user from Istio headers OR decodes JWT (Postman mode)    │
│     - Forwards SAME token to gitea-service                              │
│                           ↓                                              │
│  6. Gitea Service processes request                                      │
│     - Same token validation (already trusted by Istio)                  │
│     - Returns user's accessible repositories                             │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### JWT vs JWKS

| Component | Type | Description |
|-----------|------|-------------|
| **JWT** | Token | The actual access token (header.payload.signature) |
| **JWKS** | Endpoint | JSON Web Key Set - public keys to verify JWTs |
| **RS256** | Algorithm | RSA + SHA-256, asymmetric signing |
| **kid** | Header Field | Key ID - identifies which JWKS key to use |

### Keycloak Endpoints

```
Token:    POST http://keycloak:30080/realms/devplatform/protocol/openid-connect/token
JWKS:     GET  http://keycloak:30080/realms/devplatform/protocol/openid-connect/certs
UserInfo: GET  http://keycloak:30080/realms/devplatform/protocol/openid-connect/userinfo
```

### Token Forwarding Pattern

```go
// In GraphQL resolver - token is forwarded to downstream services
func (s *Schema) resolveProvisionCodeServer(p graphql.ResolveParams) {
    // 1. Get user from context (extracted by auth middleware)
    userID := auth.GetUserFromContext(p.Context)

    // 2. Get the SAME token from the incoming request
    token := auth.GetTokenFromContext(p.Context)

    // 3. Forward token to gitea-service (validates user's repo access)
    hasAccess, _ := s.gitea.ValidateRepoAccess(ctx, token, owner, repoName)
    //                                              ^^^^^ same JWT token

    // 4. Get clone URL (also uses forwarded token)
    cloneURL, _ := s.gitea.GetRepoCloneURL(ctx, token, owner, repoName)
}
```

### Key Points

- **One Token Everywhere**: User logs in once, same token works for all services
- **Istio Validates**: JWT validation at gateway level, services trust Istio headers
- **Token Forwarding**: Services pass the user's token to downstream services
- **No Inter-Service Secrets**: CodeServer doesn't need Keycloak client secret
- **RS256 (Asymmetric)**: Private key stays in Keycloak, public keys in JWKS

## Architecture

### System Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Istio Ingress Gateway                               │
│                    (JWT Validation + Routing)                               │
└─────────────────────────────────┬───────────────────────────────────────────┘
                                  │
        ┌─────────────────────────┼─────────────────────────┐
        │                         │                         │
        ▼                         ▼                         ▼
┌───────────────┐       ┌─────────────────┐       ┌─────────────────┐
│ CodeServer    │       │ Gitea Service   │       │ LDAP Manager    │
│ Service       │       │ (Repo Access)   │       │ (User Info)     │
│ (This one)    │       └────────┬────────┘       └────────┬────────┘
└───────┬───────┘                │                         │
        │                        ▼                         ▼
        │               ┌─────────────────┐       ┌─────────────────┐
        │               │  Gitea Server   │       │    OpenLDAP     │
        │               │  (Git repos)    │       │   (Users DB)    │
        │               └─────────────────┘       └─────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Kubernetes API                                       │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐              │
│  │ code-server-    │  │ code-server-    │  │ code-server-    │              │
│  │ {user1}         │  │ {user2}         │  │ {user3}         │  ...         │
│  │ Pod + PVC       │  │ Pod + PVC       │  │ Pod + PVC       │              │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Request Flow

```
1. User authenticates via Keycloak → Gets JWT token
2. User calls: provisionCodeServer(repoName: "my-project")
3. CodeServer Service:
   a. Validates JWT (via Istio or middleware)
   b. Queries Gitea Service for repo access validation
   c. Checks if user already has a PVC
      - No PVC: Create PVC (10Gi default)
      - Has PVC: Reuse existing
   d. Checks if Pod exists
      - No Pod: Create Pod with code-server + git clone
      - Pod exists: Return existing URL
   e. Creates/updates Istio VirtualService for routing
   f. Returns access URL: https://code-{userId}.devplatform.local
4. User opens URL → VS Code in browser with repo ready
```

## Directory Structure

```
codeserver-service/
├── cmd/server/
│   └── main.go                    # HTTP server, middleware, graceful shutdown
├── internal/
│   ├── config/
│   │   └── config.go              # Environment-based configuration
│   ├── auth/
│   │   └── middleware.go          # JWT extraction middleware
│   ├── kubernetes/
│   │   ├── client.go              # Kubernetes API client
│   │   ├── pods.go                # Pod management (create/delete/status)
│   │   ├── pvc.go                 # PersistentVolumeClaim management
│   │   ├── service.go             # Service/Ingress management
│   │   └── templates.go           # Pod/PVC/Service templates
│   ├── gitea/
│   │   └── client.go              # Gitea API client (repo validation)
│   ├── graphql/
│   │   ├── schema.go              # Main GraphQL schema
│   │   ├── types_instance.go      # CodeServerInstance types
│   │   └── resolvers.go           # Query/Mutation resolvers
│   └── models/
│       └── models.go              # Data structures
├── k8s/
│   ├── 00-namespace.yaml          # Namespace for code-server pods
│   ├── 01-configmap.yaml          # Service configuration
│   ├── 02-secret.yaml             # Secrets (JWT, Gitea token)
│   ├── 03-rbac.yaml               # ServiceAccount + RBAC for K8s API
│   ├── 04-deployment.yaml         # Service deployment
│   ├── 05-service.yaml            # Service exposure
│   ├── 06-istio-config.yaml       # Istio Gateway/VirtualService
│   └── templates/
│       ├── codeserver-pod.yaml    # Template for user pods
│       └── codeserver-pvc.yaml    # Template for user PVCs
├── Dockerfile                     # Multi-stage production build
├── Makefile                       # Common operations
├── go.mod                         # Go dependencies
└── README.md                      # Documentation
```

## Implementation Details

### 1. Configuration (internal/config/config.go)

**Required Environment Variables:**

```go
type Config struct {
    // Server
    Port            int    `envconfig:"PORT" default:"8082"`
    MetricsPort     int    `envconfig:"METRICS_PORT" default:"9092"`
    Environment     string `envconfig:"ENVIRONMENT" default:"development"`
    LogLevel        string `envconfig:"LOG_LEVEL" default:"info"`
    ShutdownTimeout int    `envconfig:"SHUTDOWN_TIMEOUT" default:"30"`

    // Kubernetes
    KubeConfig      string `envconfig:"KUBECONFIG" default:""`           // Empty = in-cluster
    Namespace       string `envconfig:"CODESERVER_NAMESPACE" default:"codeserver-instances"`
    PVCStorageClass string `envconfig:"PVC_STORAGE_CLASS" default:"standard"`
    PVCSize         string `envconfig:"PVC_SIZE" default:"10Gi"`

    // Code-Server
    CodeServerImage   string `envconfig:"CODESERVER_IMAGE" default:"codercom/code-server:latest"`
    CodeServerCPU     string `envconfig:"CODESERVER_CPU" default:"500m"`
    CodeServerMemory  string `envconfig:"CODESERVER_MEMORY" default:"1Gi"`
    CodeServerTimeout int    `envconfig:"CODESERVER_TIMEOUT" default:"300"` // Pod startup timeout

    // Gitea Integration
    GiteaURL      string `envconfig:"GITEA_URL" required:"true"`
    GiteaToken    string `envconfig:"GITEA_TOKEN" required:"true"`       // Admin token for cloning

    // Gitea Service (for access validation)
    GiteaServiceURL string `envconfig:"GITEA_SERVICE_URL" required:"true"`

    // Authentication
    JWTSecret string `envconfig:"JWT_SECRET" required:"true"`

    // Domain
    BaseDomain string `envconfig:"BASE_DOMAIN" default:"devplatform.local"`
}
```

### 2. Kubernetes Client (internal/kubernetes/client.go)

**Client Initialization:**

```go
type Client struct {
    clientset  *kubernetes.Clientset
    dynamicClient dynamic.Interface
    config     *config.Config
    logger     *logrus.Logger
}

func NewClient(cfg *config.Config, logger *logrus.Logger) (*Client, error) {
    var kubeConfig *rest.Config
    var err error

    if cfg.KubeConfig != "" {
        // Out-of-cluster (development)
        kubeConfig, err = clientcmd.BuildConfigFromFlags("", cfg.KubeConfig)
    } else {
        // In-cluster (production)
        kubeConfig, err = rest.InClusterConfig()
    }

    clientset, err := kubernetes.NewForConfig(kubeConfig)
    dynamicClient, err := dynamic.NewForConfig(kubeConfig)

    return &Client{
        clientset:     clientset,
        dynamicClient: dynamicClient,
        config:        cfg,
        logger:        logger,
    }, nil
}
```

### 3. Pod Management (internal/kubernetes/pods.go)

**Key Functions:**

```go
// CreateCodeServerPod creates a new code-server pod for a user
func (c *Client) CreateCodeServerPod(ctx context.Context, userID, repoURL, repoName string) (*corev1.Pod, error)

// GetCodeServerPod gets the current pod for a user
func (c *Client) GetCodeServerPod(ctx context.Context, userID string) (*corev1.Pod, error)

// DeleteCodeServerPod deletes a user's code-server pod
func (c *Client) DeleteCodeServerPod(ctx context.Context, userID string) error

// GetPodStatus returns the current status of a pod
func (c *Client) GetPodStatus(ctx context.Context, userID string) (string, error)

// WaitForPodReady waits for pod to be in Running state
func (c *Client) WaitForPodReady(ctx context.Context, userID string, timeout time.Duration) error

// GetPodLogs returns logs from a user's pod
func (c *Client) GetPodLogs(ctx context.Context, userID string, lines int) (string, error)
```

**Pod Template:**

```go
func (c *Client) buildCodeServerPod(userID, repoURL, repoName string, pvcName string) *corev1.Pod {
    podName := fmt.Sprintf("code-server-%s", sanitize(userID))

    return &corev1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name:      podName,
            Namespace: c.config.Namespace,
            Labels: map[string]string{
                "app":     "code-server",
                "user":    userID,
                "repo":    repoName,
                "managed": "codeserver-service",
            },
        },
        Spec: corev1.PodSpec{
            Containers: []corev1.Container{
                {
                    Name:  "code-server",
                    Image: c.config.CodeServerImage,
                    Ports: []corev1.ContainerPort{
                        {ContainerPort: 8080, Name: "http"},
                    },
                    Env: []corev1.EnvVar{
                        {Name: "PASSWORD", Value: ""}, // No password, auth via Istio
                        {Name: "GIT_REPO_URL", Value: repoURL},
                        {Name: "REPO_NAME", Value: repoName},
                    },
                    VolumeMounts: []corev1.VolumeMount{
                        {Name: "workspace", MountPath: "/home/coder/workspace"},
                    },
                    Resources: corev1.ResourceRequirements{
                        Requests: corev1.ResourceList{
                            corev1.ResourceCPU:    resource.MustParse(c.config.CodeServerCPU),
                            corev1.ResourceMemory: resource.MustParse(c.config.CodeServerMemory),
                        },
                        Limits: corev1.ResourceList{
                            corev1.ResourceCPU:    resource.MustParse("2"),
                            corev1.ResourceMemory: resource.MustParse("4Gi"),
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
                    },
                },
            },
            InitContainers: []corev1.Container{
                {
                    Name:  "git-clone",
                    Image: "alpine/git:latest",
                    Command: []string{"/bin/sh", "-c"},
                    Args: []string{
                        `if [ ! -d "/workspace/${REPO_NAME}/.git" ]; then
                            git clone ${GIT_REPO_URL} /workspace/${REPO_NAME}
                        else
                            cd /workspace/${REPO_NAME} && git pull
                        fi`,
                    },
                    Env: []corev1.EnvVar{
                        {Name: "GIT_REPO_URL", Value: repoURL},
                        {Name: "REPO_NAME", Value: repoName},
                    },
                    VolumeMounts: []corev1.VolumeMount{
                        {Name: "workspace", MountPath: "/workspace"},
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
            },
            SecurityContext: &corev1.PodSecurityContext{
                FSGroup: ptr.To(int64(1000)),
            },
        },
    }
}
```

### 4. PVC Management (internal/kubernetes/pvc.go)

**Key Functions:**

```go
// EnsurePVC creates a PVC if it doesn't exist, returns existing if it does
func (c *Client) EnsurePVC(ctx context.Context, userID string) (*corev1.PersistentVolumeClaim, error)

// GetPVC gets the PVC for a user
func (c *Client) GetPVC(ctx context.Context, userID string) (*corev1.PersistentVolumeClaim, error)

// DeletePVC deletes a user's PVC (and all their data)
func (c *Client) DeletePVC(ctx context.Context, userID string) error

// GetPVCUsage returns storage usage for a user's PVC
func (c *Client) GetPVCUsage(ctx context.Context, userID string) (string, error)
```

**PVC Template:**

```go
func (c *Client) buildPVC(userID string) *corev1.PersistentVolumeClaim {
    pvcName := fmt.Sprintf("workspace-%s", sanitize(userID))

    return &corev1.PersistentVolumeClaim{
        ObjectMeta: metav1.ObjectMeta{
            Name:      pvcName,
            Namespace: c.config.Namespace,
            Labels: map[string]string{
                "app":     "code-server",
                "user":    userID,
                "managed": "codeserver-service",
            },
        },
        Spec: corev1.PersistentVolumeClaimSpec{
            AccessModes: []corev1.PersistentVolumeAccessMode{
                corev1.ReadWriteOnce,
            },
            StorageClassName: &c.config.PVCStorageClass,
            Resources: corev1.ResourceRequirements{
                Requests: corev1.ResourceList{
                    corev1.ResourceStorage: resource.MustParse(c.config.PVCSize),
                },
            },
        },
    }
}
```

### 5. Service/Ingress Management (internal/kubernetes/service.go)

**Key Functions:**

```go
// EnsureService creates a Service for the user's pod
func (c *Client) EnsureService(ctx context.Context, userID string) (*corev1.Service, error)

// EnsureVirtualService creates/updates Istio VirtualService for routing
func (c *Client) EnsureVirtualService(ctx context.Context, userID string) error

// DeleteService removes the user's Service
func (c *Client) DeleteService(ctx context.Context, userID string) error

// GetAccessURL returns the URL to access the user's code-server
func (c *Client) GetAccessURL(userID string) string
```

**Service Template:**

```go
func (c *Client) buildService(userID string) *corev1.Service {
    serviceName := fmt.Sprintf("code-server-%s", sanitize(userID))

    return &corev1.Service{
        ObjectMeta: metav1.ObjectMeta{
            Name:      serviceName,
            Namespace: c.config.Namespace,
            Labels: map[string]string{
                "app":     "code-server",
                "user":    userID,
                "managed": "codeserver-service",
            },
        },
        Spec: corev1.ServiceSpec{
            Selector: map[string]string{
                "app":  "code-server",
                "user": userID,
            },
            Ports: []corev1.ServicePort{
                {
                    Name:       "http",
                    Port:       80,
                    TargetPort: intstr.FromInt(8080),
                },
            },
        },
    }
}
```

### 6. Gitea Client (internal/gitea/client.go)

**For Repository Access Validation:**

```go
type Client struct {
    giteaServiceURL string
    httpClient      *http.Client
    logger          *logrus.Logger
}

// ValidateRepoAccess checks if user can access the repository
func (c *Client) ValidateRepoAccess(ctx context.Context, token, owner, repoName string) (bool, error)

// GetRepoCloneURL returns the clone URL for a repository
func (c *Client) GetRepoCloneURL(ctx context.Context, token, owner, repoName string) (string, error)

// GetUserRepos returns list of repos accessible by user
func (c *Client) GetUserRepos(ctx context.Context, token string) ([]Repository, error)
```

### 7. GraphQL Schema (internal/graphql/schema.go)

**Types:**

```graphql
type CodeServerInstance {
    id: String!
    userId: String!
    repoName: String!
    repoOwner: String!
    url: String!
    status: InstanceStatus!
    createdAt: String!
    lastAccessedAt: String
    storageUsed: String
}

enum InstanceStatus {
    PENDING
    STARTING
    RUNNING
    STOPPING
    STOPPED
    ERROR
}

type ProvisionResult {
    instance: CodeServerInstance!
    message: String!
    isNew: Boolean!
}

type InstanceStats {
    totalInstances: Int!
    runningInstances: Int!
    stoppedInstances: Int!
    totalStorageUsed: String!
}
```

**Queries:**

```graphql
type Query {
    # Get current user's code-server instances
    myCodeServers: [CodeServerInstance!]!

    # Get specific instance by ID
    codeServer(id: String!): CodeServerInstance

    # Get instance status
    codeServerStatus(id: String!): InstanceStatus!

    # Get instance logs
    codeServerLogs(id: String!, lines: Int): String!

    # Health check
    health: Boolean!

    # Stats (admin only)
    instanceStats: InstanceStats!
}
```

**Mutations:**

```graphql
type Mutation {
    # Provision a new code-server instance for a repository
    provisionCodeServer(
        repoOwner: String!
        repoName: String!
    ): ProvisionResult!

    # Stop a running instance (keeps PVC)
    stopCodeServer(id: String!): Boolean!

    # Start a stopped instance
    startCodeServer(id: String!): CodeServerInstance!

    # Delete instance completely (removes PVC)
    deleteCodeServer(id: String!): Boolean!

    # Pull latest changes from repo
    syncRepository(id: String!): Boolean!
}
```

### 8. GraphQL Resolvers (internal/graphql/resolvers.go)

**Provision Flow:**

```go
func (s *Schema) resolveProvisionCodeServer(p graphql.ResolveParams) (interface{}, error) {
    // 1. Get user from context (JWT)
    userID := auth.GetUserFromContext(p.Context)
    if userID == "" {
        return nil, errors.New("unauthorized")
    }

    repoOwner := p.Args["repoOwner"].(string)
    repoName := p.Args["repoName"].(string)

    // 2. Validate repo access via Gitea Service
    token := auth.GetTokenFromContext(p.Context)
    hasAccess, err := s.giteaClient.ValidateRepoAccess(p.Context, token, repoOwner, repoName)
    if err != nil || !hasAccess {
        return nil, errors.New("repository access denied")
    }

    // 3. Get clone URL
    cloneURL, err := s.giteaClient.GetRepoCloneURL(p.Context, token, repoOwner, repoName)
    if err != nil {
        return nil, fmt.Errorf("failed to get repo URL: %w", err)
    }

    // 4. Ensure PVC exists
    pvc, err := s.k8sClient.EnsurePVC(p.Context, userID)
    if err != nil {
        return nil, fmt.Errorf("failed to create storage: %w", err)
    }

    // 5. Check if pod already exists
    existingPod, err := s.k8sClient.GetCodeServerPod(p.Context, userID)
    if err == nil && existingPod != nil {
        // Pod exists - return existing instance
        return &models.ProvisionResult{
            Instance: s.podToInstance(existingPod),
            Message:  "Using existing instance",
            IsNew:    false,
        }, nil
    }

    // 6. Create new pod
    pod, err := s.k8sClient.CreateCodeServerPod(p.Context, userID, cloneURL, repoName)
    if err != nil {
        return nil, fmt.Errorf("failed to create instance: %w", err)
    }

    // 7. Create service for the pod
    _, err = s.k8sClient.EnsureService(p.Context, userID)
    if err != nil {
        return nil, fmt.Errorf("failed to create service: %w", err)
    }

    // 8. Create/update VirtualService for routing
    err = s.k8sClient.EnsureVirtualService(p.Context, userID)
    if err != nil {
        return nil, fmt.Errorf("failed to configure routing: %w", err)
    }

    // 9. Wait for pod to be ready (with timeout)
    err = s.k8sClient.WaitForPodReady(p.Context, userID, time.Duration(s.config.CodeServerTimeout)*time.Second)
    if err != nil {
        s.logger.WithError(err).Warn("Pod not ready within timeout, returning pending status")
    }

    return &models.ProvisionResult{
        Instance: s.podToInstance(pod),
        Message:  "Instance created successfully",
        IsNew:    true,
    }, nil
}
```

### 9. HTTP Server (cmd/server/main.go)

**Setup (same pattern as gitea-service):**

```go
func main() {
    // Load config
    cfg := config.Load()

    // Setup logger
    logger := setupLogger(cfg)

    // Initialize Kubernetes client
    k8sClient, err := kubernetes.NewClient(cfg, logger)
    if err != nil {
        logger.WithError(err).Fatal("Failed to create Kubernetes client")
    }

    // Initialize Gitea client
    giteaClient := gitea.NewClient(cfg, logger)

    // Initialize GraphQL schema
    gqlSchema := graphql.NewSchema(k8sClient, giteaClient, cfg, logger)

    // Setup HTTP server with middleware
    srv := setupHTTPServer(cfg, gqlSchema, logger)

    // Start metrics server
    go startMetricsServer(cfg, logger)

    // Start main server
    go srv.ListenAndServe()

    // Wait for shutdown
    waitForShutdown(srv, cfg, logger)
}
```

**Endpoints:**
- `POST /graphql`: GraphQL API
- `GET /health`: Liveness probe
- `GET /ready`: Readiness probe (tests K8s API connection)
- `GET /metrics`: Prometheus metrics (separate port)

### 10. Kubernetes RBAC (k8s/03-rbac.yaml)

**ServiceAccount with permissions to manage pods/PVCs:**

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: codeserver-service
  namespace: dev-platform

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: codeserver-manager
rules:
  # Pod management
  - apiGroups: [""]
    resources: ["pods", "pods/log", "pods/status"]
    verbs: ["get", "list", "watch", "create", "update", "delete"]

  # PVC management
  - apiGroups: [""]
    resources: ["persistentvolumeclaims"]
    verbs: ["get", "list", "watch", "create", "update", "delete"]

  # Service management
  - apiGroups: [""]
    resources: ["services"]
    verbs: ["get", "list", "watch", "create", "update", "delete"]

  # Istio VirtualService management
  - apiGroups: ["networking.istio.io"]
    resources: ["virtualservices", "destinationrules"]
    verbs: ["get", "list", "watch", "create", "update", "delete"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: codeserver-service-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: codeserver-manager
subjects:
  - kind: ServiceAccount
    name: codeserver-service
    namespace: dev-platform
```

### 11. Deployment Manifest (k8s/04-deployment.yaml)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: codeserver-service
  namespace: dev-platform
spec:
  replicas: 2
  selector:
    matchLabels:
      app: codeserver-service
  template:
    metadata:
      labels:
        app: codeserver-service
    spec:
      serviceAccountName: codeserver-service
      containers:
        - name: codeserver-service
          image: codeserver-service:latest
          ports:
            - containerPort: 8082
              name: http
            - containerPort: 9092
              name: metrics
          envFrom:
            - configMapRef:
                name: codeserver-service-config
            - secretRef:
                name: codeserver-service-secrets
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 512Mi
          livenessProbe:
            httpGet:
              path: /health
              port: 8082
            initialDelaySeconds: 10
            periodSeconds: 30
          readinessProbe:
            httpGet:
              path: /ready
              port: 8082
            initialDelaySeconds: 5
            periodSeconds: 10
          securityContext:
            runAsNonRoot: true
            runAsUser: 1000
            readOnlyRootFilesystem: true
            allowPrivilegeEscalation: false
```

### 12. Dockerfile

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always) -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o codeserver-service ./cmd/server

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 1000 appuser

WORKDIR /app

COPY --from=builder /app/codeserver-service .

USER appuser

EXPOSE 8082 9092

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8082/health || exit 1

ENTRYPOINT ["/app/codeserver-service"]
```

## Data Models (internal/models/models.go)

```go
package models

import "time"

type InstanceStatus string

const (
    StatusPending  InstanceStatus = "PENDING"
    StatusStarting InstanceStatus = "STARTING"
    StatusRunning  InstanceStatus = "RUNNING"
    StatusStopping InstanceStatus = "STOPPING"
    StatusStopped  InstanceStatus = "STOPPED"
    StatusError    InstanceStatus = "ERROR"
)

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
}

type ProvisionResult struct {
    Instance *CodeServerInstance `json:"instance"`
    Message  string              `json:"message"`
    IsNew    bool                `json:"isNew"`
}

type InstanceStats struct {
    TotalInstances    int    `json:"totalInstances"`
    RunningInstances  int    `json:"runningInstances"`
    StoppedInstances  int    `json:"stoppedInstances"`
    TotalStorageUsed  string `json:"totalStorageUsed"`
}

type Repository struct {
    Owner    string `json:"owner"`
    Name     string `json:"name"`
    CloneURL string `json:"cloneUrl"`
    Private  bool   `json:"private"`
}
```

## API Usage Examples

### Provision a Code Server

```graphql
mutation {
  provisionCodeServer(repoOwner: "devteam", repoName: "backend-api") {
    instance {
      id
      url
      status
    }
    message
    isNew
  }
}
```

**Response:**
```json
{
  "data": {
    "provisionCodeServer": {
      "instance": {
        "id": "code-server-john-doe",
        "url": "https://code-john-doe.devplatform.local",
        "status": "RUNNING"
      },
      "message": "Instance created successfully",
      "isNew": true
    }
  }
}
```

### List My Instances

```graphql
query {
  myCodeServers {
    id
    repoName
    url
    status
    storageUsed
    createdAt
  }
}
```

### Stop Instance

```graphql
mutation {
  stopCodeServer(id: "code-server-john-doe")
}
```

### Sync Repository (Pull Latest)

```graphql
mutation {
  syncRepository(id: "code-server-john-doe")
}
```

## Prometheus Metrics

```go
var (
    instancesTotal = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "codeserver_instances_total",
            Help: "Total number of code-server instances by status",
        },
        []string{"status"},
    )

    provisionDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "codeserver_provision_duration_seconds",
            Help:    "Time to provision a code-server instance",
            Buckets: []float64{5, 10, 30, 60, 120, 300},
        },
        []string{"result"},
    )

    storageUsed = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "codeserver_storage_used_bytes",
            Help: "Storage used by user workspaces",
        },
        []string{"user"},
    )
)
```

## Error Handling

| Error | Cause | Resolution |
|-------|-------|------------|
| `unauthorized` | Missing/invalid JWT | User must authenticate via Keycloak |
| `repository access denied` | User can't access repo | Check LDAP permissions |
| `failed to create storage` | PVC creation failed | Check storage class, quotas |
| `failed to create instance` | Pod creation failed | Check resource limits, RBAC |
| `instance not found` | Invalid instance ID | Verify instance exists |
| `pod not ready` | Timeout waiting for pod | Check pod events/logs |

## Production Considerations

### Security
- RBAC: Minimal permissions for service account
- Network Policies: Isolate user pods
- Pod Security: Non-root, read-only where possible
- No password on code-server (auth via Istio JWT)

### Resource Management
- CPU/Memory limits per user pod
- Storage quotas via PVC size limits
- Idle timeout: Stop pods after inactivity
- Pod count limits per user

### Reliability
- PVC ensures data persistence
- Service restarts don't lose work
- Health checks on all components
- Graceful shutdown

### Scaling
- Service itself scales horizontally
- User pods are independent
- Consider node affinity for user pods
- Monitor cluster capacity

## Development Workflow

1. **Local Development:**
   ```bash
   # Set KUBECONFIG for local cluster access
   export KUBECONFIG=~/.kube/config
   export GITEA_URL=http://localhost:3000
   export GITEA_SERVICE_URL=http://localhost:8081
   go run cmd/server/main.go
   ```

2. **Testing:**
   ```bash
   make test
   make docker-build
   make k8s-deploy
   ```

3. **GraphQL Testing:**
   ```bash
   # Get token from Keycloak
   TOKEN=$(curl -s -X POST ...)

   # Provision instance
   curl -X POST http://localhost:8082/graphql \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"query":"mutation { provisionCodeServer(repoOwner: \"admin\", repoName: \"test\") { instance { url status } } }"}'
   ```

## Success Criteria

- [ ] Service builds without errors
- [ ] Docker image builds successfully
- [ ] RBAC allows pod/PVC/service management
- [ ] GraphQL API is accessible
- [ ] provisionCodeServer creates pod + PVC
- [ ] User can access VS Code at returned URL
- [ ] Repository is pre-cloned in workspace
- [ ] Changes persist after pod restart
- [ ] stopCodeServer stops pod (keeps PVC)
- [ ] deleteCodeServer removes all resources
- [ ] syncRepository pulls latest changes
- [ ] Health/ready probes pass
- [ ] Metrics are exposed
- [ ] Multiple users get isolated instances

## Dependencies (go.mod)

```go
module github.com/devplatform/codeserver-service

go 1.21

require (
    github.com/graphql-go/graphql v0.8.1
    github.com/graphql-go/handler v0.2.4
    github.com/golang-jwt/jwt/v5 v5.2.0
    github.com/kelseyhightower/envconfig v1.4.0
    github.com/prometheus/client_golang v1.18.0
    github.com/sirupsen/logrus v1.9.3
    k8s.io/api v0.29.0
    k8s.io/apimachinery v0.29.0
    k8s.io/client-go v0.29.0
    istio.io/client-go v1.20.0
)
```

## Quick Implementation Order

1. `go.mod` - Initialize module with dependencies
2. `internal/models/models.go` - Data structures
3. `internal/config/config.go` - Configuration
4. `internal/auth/middleware.go` - JWT extraction
5. `internal/kubernetes/client.go` - K8s client init
6. `internal/kubernetes/pvc.go` - PVC management
7. `internal/kubernetes/pods.go` - Pod management
8. `internal/kubernetes/service.go` - Service/routing
9. `internal/gitea/client.go` - Repo validation
10. `internal/graphql/types_instance.go` - GraphQL types
11. `internal/graphql/schema.go` - Schema + resolvers
12. `cmd/server/main.go` - HTTP server
13. `Dockerfile` - Container build
14. `k8s/*.yaml` - Kubernetes manifests
15. `Makefile` - Build automation
