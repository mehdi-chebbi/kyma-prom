# Repository Access Control - OAuth2 + LDAP Groups + Gitea Teams

Complete guide for managing repository access with mixed users from different departments using LDAP groups, Gitea teams, and OAuth2/Istio session management.

## Current State

### ✅ What We Have

**LDAP Manager:**
- ✅ `assignRepoToUser` - Assign repositories to individual users
- ✅ `assignRepoToDepartment` - Assign repositories to entire department
- ✅ `createGroup` - Create LDAP group (posixGroup + groupOfNames)
- ✅ `addUserToGroup` - Add user to LDAP group

**Gitea Service:**
- ✅ Repository CRUD operations
- ✅ OAuth2 authentication with Keycloak
- ✅ Issue and PR management

**OAuth2/Keycloak:**
- ✅ JWT token with user claims (preferred_username, email, roles)
- ✅ LDAP user federation
- ✅ Single sign-on across services

### ❌ What We Need

**Missing Features:**
- ❌ Assign repositories to LDAP groups
- ❌ Gitea team management (create teams, add members)
- ❌ Gitea repository collaborator management
- ❌ Sync LDAP groups → Gitea teams
- ❌ OAuth2 session-based repository access control
- ❌ Role-based permissions (read, write, admin)

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                  User Authentication Flow                    │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌──────────────────────────────────────────────────────────────┐
│                    Keycloak (OAuth2)                         │
│  - Authenticates user (john.doe)                             │
│  - Issues JWT with claims:                                   │
│    * preferred_username: john.doe                            │
│    * email: john.doe@devplatform.local                       │
│    * groups: [developers, project-alpha-team]                │
│    * realm_roles: [user, developer]                          │
└───────────────────┬──────────────────────────────────────────┘
                    │
                    ▼
┌──────────────────────────────────────────────────────────────┐
│              Istio RequestAuthentication                     │
│  - Validates JWT signature                                   │
│  - Extracts claims to headers:                               │
│    * X-Forwarded-User: john.doe                              │
│    * X-Forwarded-Email: john.doe@devplatform.local           │
│    * X-Auth-Request-Groups: developers,project-alpha-team    │
└───────────────────┬──────────────────────────────────────────┘
                    │
        ┌───────────┴──────────┐
        │                      │
        ▼                      ▼
┌──────────────┐      ┌──────────────────┐
│ LDAP Manager │      │  Gitea Service   │
│   Service    │      │                  │
│              │      │ Checks if user   │
│ Manages:     │      │ has access to    │
│ - Users      │◄─────┤ repository via:  │
│ - Groups     │      │ 1. Ownership     │
│ - Dept       │      │ 2. Team member   │
└──────────────┘      │ 3. Collaborator  │
                      └──────────────────┘
```

## Solution Design

### 1. LDAP Groups for Mixed Department Teams

**Use Case**: Project with users from multiple departments

**Example**:
- **Project**: Alpha Product
- **Team Members**:
  - john.doe (engineering)
  - alice.chen (datascience)
  - bob.wilson (devops)

**Solution**: Create LDAP group `project-alpha-team`

```graphql
# Step 1: Create project group
mutation {
  createGroup(
    cn: "project-alpha-team"
    description: "Cross-functional team for Alpha product"
  ) {
    cn
    gidNumber
    members
  }
}

# Step 2: Add users from different departments
mutation {
  addUserToGroup(uid: "john.doe", groupCn: "project-alpha-team")
  addUserToGroup(uid: "alice.chen", groupCn: "project-alpha-team")
  addUserToGroup(uid: "bob.wilson", groupCn: "project-alpha-team")
}

# Step 3: Assign repositories to group
mutation {
  assignRepoToGroup(
    groupCn: "project-alpha-team"
    repositories: [
      "https://github.com/devplatform/alpha-frontend",
      "https://github.com/devplatform/alpha-backend",
      "https://github.com/devplatform/alpha-ml-models"
    ]
  ) {
    cn
    repositories
    members
  }
}
```

### 2. Gitea Teams Integration

**Gitea API Endpoints for Teams:**
- `POST /orgs/{org}/teams` - Create team
- `POST /teams/{id}/members/{username}` - Add member to team
- `PUT /teams/{id}/repos/{owner}/{repo}` - Add repository to team
- `DELETE /teams/{id}/members/{username}` - Remove member

**New Gitea Service Mutations:**

```graphql
type Team {
  id: ID!
  name: String!
  description: String
  permission: TeamPermission!  # read, write, admin
  members: [User!]!
  repositories: [Repository!]!
}

enum TeamPermission {
  READ
  WRITE
  ADMIN
}

input CreateTeamInput {
  orgName: String!
  name: String!
  description: String
  permission: TeamPermission!
}

input AddTeamMemberInput {
  teamId: ID!
  username: String!
}

input AddTeamRepoInput {
  teamId: ID!
  owner: String!
  repo: String!
  permission: TeamPermission
}

type Mutation {
  # Create Gitea team
  createTeam(input: CreateTeamInput!): Team!

  # Add member to team
  addTeamMember(input: AddTeamMemberInput!): Team!

  # Add repository to team with permissions
  addTeamRepository(input: AddTeamRepoInput!): Team!

  # Sync LDAP group to Gitea team
  syncGroupToTeam(
    groupCn: String!
    orgName: String!
    teamName: String
    permission: TeamPermission!
  ): Team!
}

type Query {
  # List teams in organization
  listTeams(orgName: String!): [Team!]!

  # Get team details
  team(teamId: ID!): Team
}
```

### 3. Repository Collaborator Management

**For individual access grants** (outside of teams):

```graphql
enum CollaboratorPermission {
  READ      # Can pull
  WRITE     # Can push
  ADMIN     # Full access
}

input AddCollaboratorInput {
  owner: String!
  repo: String!
  username: String!
  permission: CollaboratorPermission!
}

type Mutation {
  # Add collaborator to repository
  addCollaborator(input: AddCollaboratorInput!): Boolean!

  # Remove collaborator
  removeCollaborator(owner: String!, repo: String!, username: String!): Boolean!

  # List collaborators
  listCollaborators(owner: String!, repo: String!): [Collaborator!]!
}

type Collaborator {
  user: User!
  permission: CollaboratorPermission!
}
```

### 4. LDAP Group → Gitea Team Sync

**Automated workflow to sync LDAP groups to Gitea teams:**

```graphql
mutation {
  # Creates/updates Gitea team based on LDAP group
  syncGroupToTeam(
    groupCn: "project-alpha-team"
    orgName: "devplatform"
    teamName: "Project Alpha Team"  # Optional, defaults to groupCn
    permission: WRITE
  ) {
    id
    name
    members {
      login
      email
    }
    repositories {
      name
      permission
    }
  }
}
```

**What this does:**
1. Reads LDAP group `project-alpha-team`
2. Gets all members from LDAP
3. Creates Gitea team "Project Alpha Team" in org "devplatform"
4. Adds all LDAP group members to Gitea team
5. Assigns repositories from LDAP group attribute to Gitea team
6. Sets team permission level (READ/WRITE/ADMIN)

### 5. OAuth2 Session-Based Access Control

**How it works with Istio:**

```yaml
# Istio AuthorizationPolicy
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
  name: gitea-repo-access
  namespace: dev-platform
spec:
  selector:
    matchLabels:
      app: gitea
  action: ALLOW
  rules:
    # Allow if user is in project-alpha-team group
    - when:
        - key: request.auth.claims[groups]
          values: ["project-alpha-team"]
      to:
        - operation:
            paths: ["/devplatform/alpha-*"]

    # Allow repo owners
    - when:
        - key: request.auth.claims[preferred_username]
          values: ["john.doe"]
      to:
        - operation:
            paths: ["/john.doe/*"]
```

**JWT Claims with Groups:**

```json
{
  "preferred_username": "john.doe",
  "email": "john.doe@devplatform.local",
  "groups": [
    "/developers",
    "/project-alpha-team",
    "/engineering-dept"
  ],
  "realm_access": {
    "roles": ["user", "developer"]
  },
  "resource_access": {
    "gitea-service": {
      "roles": ["repository-admin"]
    }
  }
}
```

**Keycloak Group Mapper:**

To include LDAP groups in JWT:

1. Go to Keycloak Admin Console
2. Realm: devplatform → Client: gitea-service → Mappers
3. Add **Group Membership** mapper:
   - Name: `groups`
   - Mapper Type: `Group Membership`
   - Token Claim Name: `groups`
   - Full group path: ON
   - Add to ID token: ON
   - Add to access token: ON
   - Add to userinfo: ON

### 6. Complete Access Control Flow

**Scenario**: john.doe wants to access repository `alpha-backend`

```
1. User authenticates → Keycloak
   ↓
2. Keycloak checks LDAP groups
   john.doe is member of: project-alpha-team, developers
   ↓
3. Keycloak issues JWT with groups claim
   {
     "preferred_username": "john.doe",
     "groups": ["project-alpha-team", "developers"]
   }
   ↓
4. User requests: GET /api/v1/repos/devplatform/alpha-backend
   Authorization: Bearer <JWT>
   ↓
5. Istio validates JWT, extracts groups
   ↓
6. Gitea Service checks access:
   a) Is user the owner? NO
   b) Is user a collaborator? NO
   c) Is user in a team with access? YES
      - project-alpha-team has WRITE access
   ↓
7. Access GRANTED with WRITE permission
```

## Implementation Steps

### Phase 1: LDAP Group Repository Assignment

**File**: `backend/internal/ldap/operations.go`

```go
// AssignRepositoriesToGroup assigns repositories to a group
func (m *Manager) AssignRepositoriesToGroup(ctx context.Context, cn string, repositories []string) error {
    conn, err := m.getConnection()
    if err != nil {
        return err
    }
    defer m.returnConnection(conn)

    groupDN := fmt.Sprintf("cn=%s,ou=groups,%s", cn, m.config.LDAP.BaseDN)

    modifyReq := ldap.NewModifyRequest(groupDN, nil)
    modifyReq.Replace("githubRepository", repositories)

    return conn.Modify(modifyReq)
}

// GetGroupRepositories gets repositories assigned to a group
func (m *Manager) GetGroupRepositories(ctx context.Context, cn string) ([]string, error) {
    // Query group's githubRepository attribute
}
```

**File**: `backend/internal/graphql/schema.go`

```go
// Add to mutations
"assignRepoToGroup": &graphql.Field{
    Type: groupType,
    Args: graphql.FieldConfigArgument{
        "groupCn": &graphql.ArgumentConfig{
            Type: graphql.NewNonNull(graphql.String),
        },
        "repositories": &graphql.ArgumentConfig{
            Type: graphql.NewNonNull(graphql.NewList(graphql.String)),
        },
    },
    Resolve: s.resolveAssignRepoToGroup,
},
```

### Phase 2: Gitea Team Management

**File**: `gitea-service/internal/gitea/teams.go` (NEW)

```go
package gitea

import (
    "context"
    "fmt"
)

type Team struct {
    ID          int64    `json:"id"`
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Permission  string   `json:"permission"` // read, write, admin
}

type CreateTeamRequest struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Permission  string `json:"permission"`
}

// CreateTeam creates a new team in an organization
func (c *Client) CreateTeam(ctx context.Context, orgName string, input *CreateTeamRequest) (*Team, error) {
    path := fmt.Sprintf("/api/v1/orgs/%s/teams", orgName)

    var team Team
    if err := c.makeRequest(ctx, "POST", path, input, &team); err != nil {
        return nil, err
    }

    return &team, nil
}

// AddTeamMember adds a user to a team
func (c *Client) AddTeamMember(ctx context.Context, teamID int64, username string) error {
    path := fmt.Sprintf("/api/v1/teams/%d/members/%s", teamID, username)
    return c.makeRequest(ctx, "PUT", path, nil, nil)
}

// AddTeamRepository adds a repository to a team
func (c *Client) AddTeamRepository(ctx context.Context, teamID int64, owner, repo string) error {
    path := fmt.Sprintf("/api/v1/teams/%d/repos/%s/%s", teamID, owner, repo)
    return c.makeRequest(ctx, "PUT", path, nil, nil)
}
```

**File**: `gitea-service/internal/graphql/teams_schema.go` (NEW)

```go
package graphql

import (
    "github.com/graphql-go/graphql"
)

var teamType = graphql.NewObject(graphql.ObjectConfig{
    Name: "Team",
    Fields: graphql.Fields{
        "id": &graphql.Field{
            Type: graphql.Int,
        },
        "name": &graphql.Field{
            Type: graphql.String,
        },
        "description": &graphql.Field{
            Type: graphql.String,
        },
        "permission": &graphql.Field{
            Type: graphql.String,
        },
    },
})

// Add team mutations to schema
```

### Phase 3: Group Sync Service

**File**: `gitea-service/internal/sync/group_sync.go` (NEW)

```go
package sync

import (
    "context"
    "github.com/devplatform/gitea-service/internal/gitea"
    "github.com/devplatform/gitea-service/internal/ldap"
)

type GroupSyncService struct {
    giteaClient *gitea.Client
    ldapClient  *ldap.Client
}

// SyncGroupToTeam syncs an LDAP group to a Gitea team
func (s *GroupSyncService) SyncGroupToTeam(ctx context.Context, groupCN, orgName, teamName string, permission string) (*gitea.Team, error) {
    // 1. Get LDAP group members
    group, err := s.ldapClient.GetGroup(ctx, groupCN)
    if err != nil {
        return nil, err
    }

    // 2. Create or update Gitea team
    team, err := s.giteaClient.CreateTeam(ctx, orgName, &gitea.CreateTeamRequest{
        Name:        teamName,
        Description: group.Description,
        Permission:  permission,
    })
    if err != nil {
        return nil, err
    }

    // 3. Add all group members to team
    for _, memberDN := range group.Members {
        uid := extractUIDFromDN(memberDN)
        _ = s.giteaClient.AddTeamMember(ctx, team.ID, uid)
    }

    // 4. Add group's repositories to team
    for _, repo := range group.Repositories {
        owner, repoName := parseGitHubURL(repo)
        _ = s.giteaClient.AddTeamRepository(ctx, team.ID, owner, repoName)
    }

    return team, nil
}
```

### Phase 4: Istio Authorization Policies

**File**: `k8s/auth/08-gitea-authz-policy.yaml` (NEW)

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
  name: gitea-repository-access
  namespace: dev-platform
spec:
  selector:
    matchLabels:
      app: gitea
  action: CUSTOM
  provider:
    name: "oauth2-proxy"
  rules:
    # Allow access based on JWT groups claim
    - when:
        - key: request.auth.claims[groups]
          values: ["*"]  # Any authenticated user
      to:
        - operation:
            methods: ["GET", "POST", "PUT", "DELETE"]
            paths: ["/api/v1/*"]
---
# External Authorization Service (optional)
apiVersion: v1
kind: Service
metadata:
  name: gitea-authz
  namespace: dev-platform
spec:
  selector:
    app: gitea-authz
  ports:
    - port: 9191
      targetPort: 9191
```

## Usage Examples

### Example 1: Create Project Team from Mixed Departments

```graphql
# 1. Create LDAP group
mutation {
  createGroup(
    cn: "mobile-app-team"
    description: "Mobile app development team"
  ) {
    cn
    gidNumber
  }
}

# 2. Add users from different departments
mutation {
  addToTeam1: addUserToGroup(uid: "john.doe", groupCn: "mobile-app-team")
  addToTeam2: addUserToGroup(uid: "alice.chen", groupCn: "mobile-app-team")
  addToTeam3: addUserToGroup(uid: "sara.frontend", groupCn: "mobile-app-team")
}

# 3. Assign repositories
mutation {
  assignRepoToGroup(
    groupCn: "mobile-app-team"
    repositories: [
      "https://github.com/devplatform/mobile-ios",
      "https://github.com/devplatform/mobile-android"
    ]
  ) {
    cn
    repositories
  }
}

# 4. Sync to Gitea team
mutation {
  syncGroupToTeam(
    groupCn: "mobile-app-team"
    orgName: "devplatform"
    teamName: "Mobile App Team"
    permission: WRITE
  ) {
    id
    name
    members {
      login
    }
  }
}
```

### Example 2: Grant Individual Access

```graphql
# Add single collaborator to repository
mutation {
  addCollaborator(input: {
    owner: "devplatform"
    repo: "special-project"
    username: "external.contractor"
    permission: READ
  })
}
```

### Example 3: Department-Wide Access

```graphql
# Assign repos to entire engineering department
mutation {
  assignRepoToDepartment(
    ou: "engineering"
    repositories: [
      "https://github.com/devplatform/core-platform",
      "https://github.com/devplatform/shared-libs"
    ]
  ) {
    ou
    repositories
  }
}
```

## Security Considerations

1. **JWT Validation**: Always validate JWT signature at Istio layer
2. **Group Claims**: Ensure Keycloak includes group membership in tokens
3. **Permission Levels**: Respect Gitea's permission hierarchy (read < write < admin)
4. **Audit Logging**: Log all access grant/revoke operations
5. **Least Privilege**: Default to READ permission, explicitly grant WRITE/ADMIN
6. **Token Expiration**: Short-lived tokens (5 min), require refresh
7. **Group Sync**: Periodic sync to keep Gitea teams updated with LDAP changes

## Monitoring & Observability

**Metrics to track:**
- Group sync operations (success/failure rate)
- Access denials by repository
- Team membership changes
- Token validation failures

**Jaeger tracing:**
- User auth → Group lookup → Team check → Access decision
- End-to-end latency for access checks

**Alerts:**
- Failed group sync operations
- High access denial rate
- Unauthorized access attempts

## Summary

This design provides:
✅ **Flexible team composition** - Mix users from any department
✅ **LDAP groups** - Single source of truth for team membership
✅ **Gitea teams** - Native repository access control
✅ **OAuth2 sessions** - JWT-based authentication with group claims
✅ **Istio policies** - Gateway-level authorization
✅ **Automated sync** - LDAP groups automatically sync to Gitea teams
✅ **Multiple access patterns** - Individual, team, or department-wide

Next: Implement the missing mutations and sync logic!
