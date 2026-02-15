# Gitea Service Architecture

## Overview

This service provides a **centralized Git repository management system** with authentication and authorization handled externally via LDAP, not Gitea's built-in user system.

## Architecture Pattern: Centralized Admin with External Auth

```
┌─────────────────────────────────────────────────────────────────┐
│                         Client Application                      │
│                    (Sends JWT Token with requests)              │
└────────────────────────────┬────────────────────────────────────┘
                             │ GraphQL Query/Mutation + JWT
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Gitea Service (This)                       │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │ 1. Validate JWT Token (extract user from LDAP Manager)   │ │
│  │ 2. Check LDAP permissions (user repos + dept repos)      │ │
│  │ 3. Execute operation as admin123 if authorized           │ │
│  └───────────────────────────────────────────────────────────┘ │
└──────────────┬──────────────────────────────┬───────────────────┘
               │                              │
               │                              │
               ▼                              ▼
┌──────────────────────────┐    ┌────────────────────────────────┐
│   LDAP Manager Service   │    │         Gitea Server           │
│  - User authentication   │    │  - Single admin user: admin123 │
│  - User repositories     │    │  - All repos owned by admin123 │
│  - Department repos      │    │  - No Gitea user management    │
└──────────────────────────┘    └────────────────────────────────┘
```

## Why This Architecture?

### ✅ Advantages

1. **Single Source of Truth**: LDAP is the only authentication/authorization system
2. **No User Sync**: No need to create/update/delete users in Gitea
3. **Full Control**: All access logic in your hands via LDAP attributes
4. **Simplified Management**: Only maintain one admin user in Gitea
5. **Security**: JWT-based authentication with LDAP validation
6. **Scalability**: Easy to add new users via LDAP without touching Gitea

### ⚠️ Trade-offs

1. Users don't have personal Gitea UI logins
2. All repos appear under "admin123" in Gitea UI
3. Can't use Gitea's native permission system
4. Gitea UI is not the primary interface (GraphQL API is)

## How It Works

### 1. Authentication Flow

```
User Request → JWT Token → Gitea Service validates with LDAP Manager
                                    ↓
                         Extracts user info (UID, department, repos)
                                    ↓
                         Checks if user can access resource
                                    ↓
                         Executes as admin123 if authorized
```

### 2. Repository Ownership

**All repositories are owned by the default admin user** (`admin123` by default)

- Configured via `GITEA_DEFAULT_OWNER` environment variable
- Can be changed if your admin user has a different name
- Falls back to `admin123` if not configured

### 3. Access Control

Access is determined by **LDAP attributes**:

- **User's personal repositories**: `githubRepository` attribute on user object
- **Department repositories**: `githubRepository` attribute on department object

Example LDAP structure:
```ldap
# User: john.doe
dn: uid=john.doe,ou=users,dc=devplatform,dc=local
uid: john.doe
departmentNumber: engineering
githubRepository: myproject
githubRepository: personal-repo

# Department: engineering
dn: ou=engineering,ou=departments,dc=devplatform,dc=local
ou: engineering
githubRepository: team-backend
githubRepository: team-frontend
```

**john.doe** can access:
- `myproject` (personal)
- `personal-repo` (personal)
- `team-backend` (department)
- `team-frontend` (department)

## Usage Examples

### Creating a Repository (Auto-default owner)

```graphql
mutation {
  createRepository(
    name: "my-new-repo"
    description: "My awesome project"
    private: true
    autoInit: true
    license: "MIT"
  ) {
    id
    name
    fullName  # Will be "admin123/my-new-repo"
    cloneUrl
  }
}
```

### Creating a Repository (Explicit owner)

```graphql
mutation {
  createRepository(
    owner: "admin123"  # Optional, same as default
    name: "my-new-repo"
    description: "My awesome project"
  ) {
    id
    name
    cloneUrl
  }
}
```

### Migrating from GitHub (Auto-default owner)

```graphql
mutation {
  migrateRepository(
    cloneAddr: "https://github.com/facebook/react"
    repoName: "react-mirror"
    service: "github"
    mirror: false
    private: false
    wiki: true
    issues: true
    pullRequests: true
    releases: true
  ) {
    id
    name
    fullName  # Will be "admin123/react-mirror"
  }
}
```

### Migrating from Private GitHub Repo

```graphql
mutation {
  migrateRepository(
    cloneAddr: "https://github.com/myorg/private-repo"
    repoName: "private-project"
    service: "github"
    private: true
    authToken: "ghp_xxxxxxxxxxxxxxxxxxxx"
    wiki: true
    issues: true
    pullRequests: true
  ) {
    id
    name
  }
}
```

## Configuration

### Environment Variables

```bash
# Gitea connection
GITEA_URL=http://gitea.dev-platform.svc.cluster.local:3000
GITEA_TOKEN=your-admin123-token

# Default owner for all repositories
GITEA_DEFAULT_OWNER=admin123  # Change if your admin user is different

# LDAP Manager
LDAP_MANAGER_URL=http://ldap-manager.dev-platform.svc.cluster.local:8080

# JWT (must match LDAP Manager)
JWT_SECRET=your-jwt-secret

# Server
PORT=8081
```

### Changing the Default Owner

If your Gitea admin user is not `admin123`:

1. **Option 1**: Set environment variable
   ```bash
   GITEA_DEFAULT_OWNER=myadmin
   ```

2. **Option 2**: Specify in each mutation
   ```graphql
   mutation {
     createRepository(
       owner: "myadmin"
       name: "my-repo"
     ) { ... }
   }
   ```

## Alternative Architectures

### Option 1: Current (Recommended for Your Case) ✅

**What**: Single admin user owns all repos, LDAP handles access

**When to use**:
- Users don't need Gitea UI access
- You want full control over permissions
- Simple to maintain

### Option 2: Sync LDAP Users to Gitea

**What**: Create Gitea users that mirror LDAP users

**Pros**: Native Gitea experience, users see "their" repos

**Cons**: Complex sync logic, duplicate user management

**Implementation**:
```go
// Would need to add:
- CreateGiteaUser(ldapUser) when new LDAP user
- UpdateGiteaUser(ldapUser) when LDAP user changes
- DeleteGiteaUser(uid) when LDAP user deleted
- Periodic sync job
```

### Option 3: Organizations for Departments

**What**: Use admin123 + Gitea organizations for LDAP departments

**Pros**: Better organization, repos grouped by department

**Cons**: Need to manage organizations

**Implementation**:
```go
// Would need to:
- Create Gitea organization per LDAP department
- Repos created under organization (e.g., engineering/backend)
- Still use admin123 for operations
```

## Security Considerations

1. **JWT Validation**: Always validates with LDAP Manager
2. **Repository Access**: Checked against LDAP attributes before every operation
3. **Admin Token**: The `GITEA_TOKEN` should be protected (K8s secret)
4. **HTTPS**: Use HTTPS for production deployments
5. **Token Expiration**: JWT tokens expire based on LDAP Manager configuration

## Troubleshooting

### "Access Denied" Errors

**Check**:
1. User's JWT token is valid
2. User has the repository in their LDAP `githubRepository` attribute
3. OR user's department has the repository

### "Repository Not Found"

**Check**:
1. Repository exists in Gitea under the default owner
2. Repository name format: `owner/repo` or just `repo`

### "Failed to Create Repository"

**Check**:
1. `GITEA_DEFAULT_OWNER` exists in Gitea
2. `GITEA_TOKEN` has admin permissions
3. Repository name doesn't already exist

## Best Practices

1. **Use Environment Variables**: Configure `GITEA_DEFAULT_OWNER` for flexibility
2. **Repository Naming**: Use clear, descriptive names
3. **LDAP Attributes**: Keep `githubRepository` attributes updated
4. **Logging**: Monitor logs for access patterns and errors
5. **Backups**: Regular backups of Gitea data (repos are valuable!)

## Migration Guide (If Switching Patterns)

### From Current to User Sync Pattern

```bash
1. Create migration script to create Gitea users
2. Transfer repository ownership
3. Update access control logic
4. Test thoroughly
5. Deploy
```

### From Current to Organizations Pattern

```bash
1. Create organizations for each department
2. Create repos under organizations
3. Update GraphQL resolvers to use organizations
4. Update client code
5. Deploy
```

## Summary

Your current architecture is **well-suited for your use case**:
- ✅ Centralized control
- ✅ LDAP as single source of truth
- ✅ Simple to maintain
- ✅ Secure with JWT validation

The auto-default to `admin123` simplifies the API while maintaining flexibility for future changes.
