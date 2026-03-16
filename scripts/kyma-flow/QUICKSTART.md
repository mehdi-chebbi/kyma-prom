# kyma-flow CLI - Quick Start Guide

## What Was Created

A new `kyma-flow` CLI tool that provides quick access to platform metrics from LDAP Manager, Gitea Service, and CodeServer Service.

## Files Created

```
scripts/kyma-flow/
├── kyma-flow.sh              # The bash script
├── kyma-flow-configmap.yaml  # Kubernetes ConfigMap
├── deploy.sh                 # Deployment script
├── README.md                 # Full documentation
└── QUICKSTART.md            # This file
```

## Files Modified

```
helm/devplatform/charts/codeserver/templates/deployment.yaml
  - Added volumeMount for kyma-flow CLI
  - Added tmp-dir volume (for /tmp access with readOnlyRootFilesystem)
  - Added kyma-flow-cli volume (ConfigMap)
```

## Deployment Steps

### Option 1: Use the Deploy Script (Recommended)

```bash
cd /home/z/my-project/project/scripts/kyma-flow
bash deploy.sh
```

### Option 2: Manual Deployment

```bash
# 1. Apply the ConfigMap
kubectl apply -f scripts/kyma-flow/kyma-flow-configmap.yaml

# 2. Restart the CodeServer Service
kubectl rollout restart deployment/codeserver-service -n dev-platform

# 3. Wait for rollout to complete
kubectl rollout status deployment/codeserver-service -n dev-platform
```

## Usage

After deployment, access kyma-flow from a CodeServer pod:

```bash
# Enter the pod
kubectl exec -it deployment/codeserver-service -n dev-platform -- bash

# Create an alias for convenience
alias kyma-flow='/opt/kyma-flow/kyma-flow'

# Run commands
kyma-flow user
kyma-flow repo
kyma-flow cloned-repo
kyma-flow storage
kyma-flow active-workspace
kyma-flow group
kyma-flow summary
kyma-flow help
```

## Commands Overview

| Command | Description | Example Output |
|---------|-------------|----------------|
| `user` | Total users in LDAP | `👥 Users: 42` |
| `group` | Total groups in LDAP | `👥 Groups: 12` |
| `repo` | Total repositories | `📦 Repositories: 156` |
| `cloned-repo` | Total migrated repos | `🔄 Migrated Repos: 23` |
| `active-workspace` | Active workspaces | `💻 Active Workspaces: 8` |
| `storage` | Storage used | `💾 Storage Used: 48 GB` |
| `summary` | All metrics at once | Grouped by service |
| `help` | Help information | Command list |

## Example Output

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

## Troubleshooting

### Check if ConfigMap exists
```bash
kubectl get configmap kyma-flow-cli -n dev-platform
```

### Check if volume is mounted
```bash
kubectl describe pod <codeserver-pod> -n dev-platform | grep -A 5 "Mounts"
```

### Verify script is accessible
```bash
kubectl exec -it deployment/codeserver-service -n dev-platform -- ls -la /opt/kyma-flow/
```

### Test service connectivity
```bash
kubectl exec -it deployment/codeserver-service -n dev-platform -- curl ldap-manager.dev-platform.svc.cluster.local:9090/health
```

## Next Steps

1. Deploy the CLI using one of the methods above
2. Test with `kyma-flow help` and `kyma-flow summary`
3. (Optional) Scale to other services by adding the same volume mount

## Notes

- No Dockerfiles were modified
- No build scripts were modified
- The script is mounted from a ConfigMap (easy to update)
- Uses Kubernetes DNS for service discovery
- Graceful error handling (continues even if one service is down)
- Storage values are human-readable (auto-converts bytes to KB/MB/GB)
