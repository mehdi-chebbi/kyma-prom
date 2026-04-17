# Gitea Integration Guide

## Overview

The LDAP Manager backend now integrates with Gitea to provide **repository access control based on LDAP attributes**. Users can only see repositories that are explicitly assigned to them via LDAP `githubRepository` attributes.

## How It Works

### Access Control Model

Users can access a Gitea repository if:

1. **Personal Assignment**: The repository is in their LDAP `githubRepository` attribute
2. **Department Assignment**: The repository is in their department's `githubRepository` attribute

### Example LDAP Structure

```ldif
# User with assigned repositories
dn: uid=john.doe,ou=users,dc=devplatform,dc=local
objectClass: inetOrgPerson
objectClass: posixAccount
objectClass: extensibleObject
uid: john.doe
cn: John Doe
mail: john@example.com
departmentNumber: engineering
githubRepository: company/frontend-app
githubRepository: company/shared-lib

# Department with assigned repositories
dn: ou=engineering,ou=departments,dc=devplatform,dc=local
objectClass: organizationalUnit
objectClass: extensibleObject
ou: engineering
description: Engineering Department
githubRepository: company/backend-api
githubRepository: company/infrastructure
```

### Repository Matching

The service normalizes repository names to handle multiple formats:

- `owner/repo` → matches exactly
- `https://github.com/owner/repo` → extracts to `owner/repo`
- `https://gitea.example.com/owner/repo` → extracts to `owner/repo`
- `repo` → matches by name only

## Environment Variables

Add these to your configuration:

```bash
# Gitea API Configuration
GITEA_URL=http://gitea.dev-platform.svc.cluster.local:3000
GITEA_TOKEN=your-gitea-admin-token-here

# Existing LDAP configuration
LDAP_URL=ldap://openldap.dev-platform.svc.cluster.local:389
LDAP_BASE_DN=dc=devplatform,dc=local
LDAP_BIND_DN=cn=admin,dc=devplatform,dc=local
LDAP_BIND_PASSWORD=admin123

# JWT Secret
JWT_SECRET=your-super-secret-jwt-key
```

### Getting Gitea Admin Token

1. Login to Gitea as admin user
2. Navigate to: **Settings → Applications**
3. Click **Generate New Token**
4. Give it a name (e.g., "LDAP Manager")
5. Select all scopes or at minimum: `repo`, `admin:org`
6. Click **Generate Token**
7. Copy the token (starts with a long alphanumeric string)
8. Set it as `GITEA_TOKEN` environment variable

## GraphQL API

### New Queries

#### Get My Accessible Repositories

```graphql
query MyRepositories {
  myGiteaRepositories {
    id
    name
    fullName
    description
    htmlUrl
    cloneUrl
    private
    language
    stars
    forks
    defaultBranch
    owner {
      login
      fullName
    }
  }
}
```

**Returns**: Only repositories assigned to the user via LDAP

#### Get Specific Repository

```graphql
query GetRepository {
  giteaRepository(owner: "company", name: "frontend-app") {
    id
    name
    description
    htmlUrl
    language
  }
}
```

**Returns**: Repository details if user has access, error otherwise

#### Search Repositories

```graphql
query SearchRepos {
  searchGiteaRepositories(query: "frontend", limit: 10) {
    name
    fullName
    description
    htmlUrl
  }
}
```

**Returns**: Matching repositories from user's accessible list

#### Repository Statistics

```graphql
query RepoStats {
  giteaRepositoryStats {
    totalCount
    privateCount
    publicCount
    languages {
      language
      count
    }
  }
}
```

**Returns**: Statistics about user's accessible repositories

### Using with Authentication

All queries require JWT authentication:

```bash
# 1. Login first
curl -X POST http://localhost:8080/graphql \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation { login(uid: \"john.doe\", password: \"password123\") { token user { uid } } }"
  }'

# 2. Use token in subsequent requests
curl -X POST http://localhost:8080/graphql \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "query": "query { myGiteaRepositories { name fullName htmlUrl } }"
  }'
```

## Admin Operations

### Assign Repository to User

Use the existing LDAP mutation:

```graphql
mutation AssignRepoToUser {
  assignRepoToUser(
    uid: "john.doe"
    repositories: ["company/frontend-app", "company/shared-lib"]
  ) {
    uid
    repositories
  }
}
```

### Assign Repository to Department

```graphql
mutation AssignRepoToDepartment {
  assignRepoToDepartment(
    ou: "engineering"
    repositories: ["company/backend-api", "company/infrastructure"]
  ) {
    ou
    repositories
  }
}
```

## Deployment

### Local Development

```bash
# 1. Start OpenLDAP (if not already running)
# Follow existing LDAP setup instructions

# 2. Start Gitea
docker run -d \
  --name gitea \
  -p 3000:3000 \
  -e USER_UID=1000 \
  -e USER_GID=1000 \
  gitea/gitea:latest

# 3. Configure Gitea and get admin token
# Visit http://localhost:3000

# 4. Set environment variables
export GITEA_URL=http://localhost:3000
export GITEA_TOKEN=your-token-here
export LDAP_URL=ldap://localhost:389
export LDAP_BASE_DN=dc=devplatform,dc=local
export LDAP_BIND_DN=cn=admin,dc=devplatform,dc=local
export LDAP_BIND_PASSWORD=admin123
export JWT_SECRET=dev-secret

# 5. Run the backend
cd backend
go run cmd/server/main.go
```

### Kubernetes Deployment

Update your backend deployment with Gitea configuration:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ldap-manager-secret
  namespace: dev-platform
type: Opaque
stringData:
  gitea-token: "your-gitea-admin-token"
  jwt-secret: "your-jwt-secret"
  ldap-bind-password: "admin123"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ldap-manager-config
  namespace: dev-platform
data:
  GITEA_URL: "http://gitea.dev-platform.svc.cluster.local:3000"
  LDAP_URL: "ldap://openldap.dev-platform.svc.cluster.local:389"
  LDAP_BASE_DN: "dc=devplatform,dc=local"
  LDAP_BIND_DN: "cn=admin,dc=devplatform,dc=local"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ldap-manager
  namespace: dev-platform
spec:
  template:
    spec:
      containers:
      - name: ldap-manager
        envFrom:
        - configMapRef:
            name: ldap-manager-config
        env:
        - name: GITEA_TOKEN
          valueFrom:
            secretKeyRef:
              name: ldap-manager-secret
              key: gitea-token
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: ldap-manager-secret
              key: jwt-secret
        - name: LDAP_BIND_PASSWORD
          valueFrom:
            secretKeyRef:
              name: ldap-manager-secret
              key: ldap-bind-password
```

## Health Checks

The `/ready` endpoint now checks both LDAP and Gitea:

```bash
curl http://localhost:8080/ready
```

**Response:**
```json
{
  "status": "ready",
  "ldap": true,
  "gitea": true
}
```

## Troubleshooting

### User Can't See Any Repositories

**Check 1**: Verify user has repositories assigned in LDAP

```bash
ldapsearch -x -H ldap://localhost:389 \
  -D "cn=admin,dc=devplatform,dc=local" \
  -w admin123 \
  -b "uid=john.doe,ou=users,dc=devplatform,dc=local" \
  githubRepository
```

**Check 2**: Verify department has repositories

```bash
ldapsearch -x -H ldap://localhost:389 \
  -D "cn=admin,dc=devplatform,dc=local" \
  -w admin123 \
  -b "ou=engineering,ou=departments,dc=devplatform,dc=local" \
  githubRepository
```

**Check 3**: Verify repositories exist in Gitea

```bash
curl http://localhost:3000/api/v1/repos/search \
  -H "Authorization: token YOUR_GITEA_TOKEN"
```

### Gitea Connection Failed

**Error**: `Gitea readiness check failed`

**Solution**:
1. Verify Gitea is running: `curl http://localhost:3000/api/v1/version`
2. Check GITEA_TOKEN is valid
3. Ensure token has correct permissions
4. Verify GITEA_URL is accessible from backend

### Repository Name Mismatch

**Issue**: Repository is assigned but user can't see it

**Cause**: Repository name format mismatch

**Solution**: Use consistent format in LDAP:
- Prefer: `owner/repo` (e.g., `company/frontend-app`)
- Avoid: Full URLs unless necessary
- The service normalizes URLs automatically, but exact `owner/repo` format is most reliable

### JWT Token Expired

**Error**: `unauthorized` or `invalid token`

**Solution**: Re-login to get a new token:

```graphql
mutation {
  login(uid: "john.doe", password: "password123") {
    token
  }
}
```

## Architecture Details

### Files Added

```
backend/
├── internal/
│   └── gitea/
│       ├── client.go          # Gitea API client
│       └── service.go          # Repository filtering logic
```

### Files Modified

```
backend/
├── cmd/server/main.go          # Initialize Gitea client/service
├── internal/
│   ├── config/config.go        # Add Gitea configuration
│   ├── models/models.go        # Add Gitea repository models
│   └── graphql/schema.go       # Add Gitea GraphQL queries
```

### Data Flow

```
User → GraphQL API (with JWT)
  ↓
Check user's LDAP attributes (repositories)
  ↓
Check user's department LDAP attributes (repositories)
  ↓
Fetch all repositories from Gitea
  ↓
Filter: Keep only repositories matching LDAP attributes
  ↓
Return filtered list to user
```

## Best Practices

1. **Use Department Assignments**: Assign repositories to departments rather than individual users when possible
2. **Consistent Naming**: Use `owner/repo` format in LDAP for clarity
3. **Token Security**: Store Gitea token securely (Kubernetes Secrets, not ConfigMaps)
4. **Regular Audits**: Review repository assignments periodically
5. **Health Monitoring**: Monitor `/ready` endpoint to ensure Gitea connectivity

## Example Workflow

### 1. Create User in LDAP

```bash
ldapadd -x -H ldap://localhost:389 \
  -D "cn=admin,dc=devplatform,dc=local" \
  -w admin123 <<EOF
dn: uid=alice.dev,ou=users,dc=devplatform,dc=local
objectClass: inetOrgPerson
objectClass: posixAccount
objectClass: extensibleObject
uid: alice.dev
cn: Alice Developer
sn: Developer
givenName: Alice
mail: alice@example.com
departmentNumber: engineering
uidNumber: 10001
gidNumber: 10001
homeDirectory: /home/alice.dev
userPassword: password123
githubRepository: company/frontend-app
EOF
```

### 2. User Logs In

```graphql
mutation {
  login(uid: "alice.dev", password: "password123") {
    token
    user {
      uid
      department
      repositories
    }
  }
}
```

### 3. User Queries Accessible Repositories

```graphql
query {
  myGiteaRepositories {
    name
    fullName
    htmlUrl
  }
}
```

**Result**: Alice sees:
- `company/frontend-app` (personal assignment)
- Any repositories assigned to `engineering` department

## Future Enhancements

Potential improvements:
- Group-based repository assignments
- Time-based access (temporary repository access)
- Webhooks for real-time repository updates
- Caching layer for performance
- Repository access audit logs
