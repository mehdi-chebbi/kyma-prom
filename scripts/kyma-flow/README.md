# kyma-flow CLI

A command-line interface for viewing KymaFlow platform metrics from LDAP Manager, Gitea Service, and CodeServer Service.

## Overview

`kyma-flow` provides quick access to business-level metrics across your KymaFlow platform. It queries the Prometheus `/metrics` endpoints of each service and displays the data in a user-friendly format.

## Installation

The kyma-flow CLI is deployed as a ConfigMap and mounted into the CodeServer Service pods.

### Deploy the ConfigMap

```bash
kubectl apply -f scripts/kyma-flow/kyma-flow-configmap.yaml
```

### Apply the Helm Chart

The CodeServer Service deployment template has been updated to mount the kyma-flow CLI. Apply or upgrade your Helm release:

```bash
cd helm
helm upgrade --install devplatform ./devplatform -f ./devplatform/values-dev.yaml
```

### Restart the CodeServer Service

```bash
kubectl rollout restart deployment/codeserver-service -n dev-platform
```

## Usage

Access the kyma-flow CLI from within a CodeServer pod:

```bash
# Enter a CodeServer pod
kubectl exec -it deployment/codeserver-service -n dev-platform -- bash

# Run kyma-flow commands
/opt/kyma-flow/kyma-flow <command>
```

### Create an Alias (Optional)

For easier use, add an alias in your shell:

```bash
alias kyma-flow='/opt/kyma-flow/kyma-flow'
```

Now you can simply run:

```bash
kyma-flow user
kyma-flow summary
```

## Available Commands

| Command | Description | Metric Source |
|---------|-------------|---------------|
| `user` | Show total users in LDAP | `ldap_users_total` |
| `group` | Show total groups in LDAP | `ldap_groups_total` |
| `repo` | Show total repositories in Gitea | `gitea_repos_total` |
| `cloned-repo` | Show total migrated repositories | `gitea_repos_migrated_total` |
| `active-workspace` | Show active workspaces | `codeserver_workspaces_active_total` |
| `storage` | Show total storage used | `codeserver_pvc_total_bytes` |
| `summary` | Show all metrics grouped by service | All metrics |
| `help` | Show help message | N/A |

## Examples

### Single Metric Commands

```bash
$ kyma-flow user
👥 Users: 42

$ kyma-flow repo
📦 Repositories: 156

$ kyma-flow storage
💾 Storage Used: 48 GB

$ kyma-flow cloned-repo
🔄 Migrated Repos: 23
```

### Summary Command

```bash
$ kyma-flow summary

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  📋 KYMA FLOW PLATFORM SUMMARY
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🔐 LDAP Manager
   👥 Users: 42
   👥 Groups: 12

📦 Gitea Service
   📦 Repositories: 156
   🔄 Migrated Repos: 23

💻 CodeServer Service
   💻 Active Workspaces: 8
   💾 Storage Used: 48 GB

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

### Help Command

```bash
$ kyma-flow help

kyma-flow - CLI for KymaFlow platform metrics

Usage: kyma-flow <command>

Available commands:
  user              Show total users
  group             Show total groups
  repo              Show total repositories
  cloned-repo       Show total migrated repositories
  active-workspace  Show active workspaces
  storage           Show total storage used
  summary           Show all metrics grouped by service
  help              Show this help message
```

## Error Handling

If a service is unreachable, kyma-flow will display an error for that metric but continue processing other metrics:

```bash
$ kyma-flow summary

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  📋 KYMA FLOW PLATFORM SUMMARY
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🔐 LDAP Manager
   👥 Users: 42
   👥 Groups: 12

📦 Gitea Service
   📦 Repositories: ❌ Service unreachable
   🔄 Migrated Repos: ❌ Service unreachable

💻 CodeServer Service
   💻 Active Workspaces: 8
   💾 Storage Used: 48 GB

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

## Configuration

### Service Endpoints

The CLI uses Kubernetes DNS to reach services. Default endpoints:

- **LDAP Manager**: `ldap-manager.dev-platform.svc.cluster.local:9090`
- **Gitea Service**: `gitea-service.dev-platform.svc.cluster.local:9091`
- **CodeServer Service**: `codeserver-service.dev-platform.svc.cluster.local:9092`

### Override Service URLs

You can override service URLs using environment variables:

```bash
export KYMA_FLOW_LDAP_MANAGER="custom-ldap-manager:9090"
export KYMA_FLOW_GITEA_SERVICE="custom-gitea-service:9091"
export KYMA_FLOW_CODESERVER_SERVICE="custom-codeserver-service:9092"

kyma-flow user
```

## Updating the CLI

To update the kyma-flow CLI:

1. Edit the script in `scripts/kyma-flow/kyma-flow.sh`
2. Update the ConfigMap:

```bash
kubectl create configmap kyma-flow-cli \
  --from-file=scripts/kyma-flow/kyma-flow.sh=kyma-flow \
  --namespace=dev-platform \
  --dry-run=client -o yaml | kubectl apply -f -
```

3. The updated script will be available immediately (no pod restart needed for new executions)

## Troubleshooting

### Command Not Found

If `kyma-flow` is not found:

```bash
# Check if the ConfigMap exists
kubectl get configmap kyma-flow-cli -n dev-platform

# Check if the volume is mounted
kubectl describe pod <codeserver-pod-name> -n dev-platform | grep -A 5 "Mounts"

# Verify the file exists
kubectl exec -it <codeserver-pod-name> -n dev-platform -- ls -la /opt/kyma-flow/
```

### Service Unreachable Errors

If you see "Service unreachable" errors:

```bash
# Check if services are running
kubectl get pods -n dev-platform

# Check service connectivity from within the pod
kubectl exec -it <codeserver-pod-name> -n dev-platform -- curl ldap-manager.dev-platform.svc.cluster.local:9090/health
kubectl exec -it <codeserver-pod-name> -n dev-platform -- curl gitea-service.dev-platform.svc.cluster.local:9091/health
kubectl exec -it <codeserver-pod-name> -n dev-platform -- curl codeserver-service.dev-platform.svc.cluster.local:9092/health

# Verify metrics endpoints
kubectl exec -it <codeserver-pod-name> -n dev-platform -- curl ldap-manager.dev-platform.svc.cluster.local:9090/metrics | grep ldap_users_total
```

### Permission Denied

If you get "Permission denied" when running the script:

```bash
# Check file permissions
kubectl exec -it <codeserver-pod-name> -n dev-platform -- ls -la /opt/kyma-flow/kyma-flow

# Run with bash explicitly
kubectl exec -it <codeserver-pod-name> -n dev-platform -- bash /opt/kyma-flow/kyma-flow user
```

## Scaling to Other Services

To make kyma-flow available in other services, add the same volume and volumeMount to their deployment templates:

```yaml
# In the container spec
volumeMounts:
- name: kyma-flow-cli
  mountPath: /opt/kyma-flow

# In the pod spec
volumes:
- name: kyma-flow-cli
  configMap:
    name: kyma-flow-cli
    defaultMode: 0755
    optional: false
```

## Architecture

```
┌─────────────────────────────────────────┐
│  ConfigMap: kyma-flow-cli               │
│  └─ kyma-flow (bash script)            │
└──────────────┬──────────────────────────┘
               │ mounted to /opt/kyma-flow/
               ▼
┌─────────────────────────────────────────┐
│  CodeServer Service Pod                │
│  └─ /opt/kyma-flow/kyma-flow          │
└──────────────┬──────────────────────────┘
               │ HTTP requests
               ▼
┌─────────────────────────────────────────┐
│  Service Metrics Endpoints              │
│  ├─ ldap-manager:9090/metrics         │
│  ├─ gitea-service:9091/metrics        │
│  └─ codeserver-service:9092/metrics   │
└─────────────────────────────────────────┘
```

## Contributing

To add new metrics:

1. Identify the metric name in the service's `/metrics` endpoint
2. Add a new command function in `kyma-flow.sh`
3. Add the command to the `main()` case statement
4. Update the `cmd_summary()` function if needed
5. Update this README with the new command
6. Update the ConfigMap

## License

Part of the KymaFlow platform.
