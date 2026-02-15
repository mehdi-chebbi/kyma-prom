# Collaboration Workflow — LDAP + Gitea + Controller

This document describes the group-based collaboration system with drag-and-drop UI.
All access is managed through **LDAP groups synced to Gitea teams**.
LDAP is the single source of truth.

---

## Design Principle

**Everything is a group.** No individual collaborator permissions.
Drag-and-drop actions create/modify LDAP groups that the controller syncs to Gitea teams.

```
┌──────────────────────────────────────────────────────────────┐
│                     SINGLE SOURCE OF TRUTH                   │
│                                                              │
│   LDAP Group/Department                                      │
│     members: [alice, bob, ...]                               │
│     repositories: [api-gateway, auth-service, ...]           │
│     baseDepartment: "engineering"  (optional, for dynamic)   │
│     extraMembers: ["dave", "eve"] (optional, for dynamic)    │
│                                                              │
│              │                                               │
│              │  Controller SyncGroupToTeam()                  │
│              ▼                                               │
│                                                              │
│   Gitea Team (mirror of LDAP group)                          │
│     permission: write                                        │
│     members: [alice, bob, ...]                               │
│     repos: [api-gateway, auth-service, ...]                  │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

---

## Three Drag-and-Drop Scenarios

### Scenario 1: Drop existing group onto a repo

User drags "backend-devs" group onto repo "api-gateway".

```
Frontend action:
    Drag group chip → drop on repo card

API call:
    mutation { addRepoToGroup(groupCN: "backend-devs", repo: "api-gateway") }

Backend flow:
    1. LDAP Manager: add "api-gateway" to backend-devs.repositories
    2. Immediate: SyncGroupToTeam("backend-devs", orgName, "backend-devs", "write")
    3. Gitea: backend-devs team now has access to api-gateway

Result:
    All members of backend-devs can push/pull api-gateway
```

### Scenario 2: Drop full department onto a repo

User drags department "engineering" onto repo "shared-lib".

```
Frontend action:
    Drag department chip → drop on repo card

API call:
    mutation { addRepoToDepartment(ou: "engineering", repo: "shared-lib") }

Backend flow:
    1. LDAP Manager: add "shared-lib" to engineering.repositories
    2. Immediate: SyncGroupToTeam("engineering", orgName, "engineering", "write")
    3. Gitea: engineering team now has access to shared-lib

Result:
    All members of engineering department can push/pull shared-lib
    New hires added to engineering automatically get access (next sync cycle)
```

### Scenario 3: Drop department + extra members onto a repo (new collab group)

User drags "engineering" + picks "dave" and "eve" onto repo "new-project".

```
Frontend action:
    1. Drag department chip → drop on repo card
    2. UI shows "Add extra members?" panel
    3. User drags dave and eve into the panel
    4. User confirms → creates collab group

API call:
    mutation {
        createCollabGroup(
            name: "collab-new-project"
            baseDepartment: "engineering"
            extraMembers: ["dave", "eve"]
            repositories: ["new-project"]
        )
    }

Backend flow:
    1. LDAP Manager: create group "collab-new-project"
       - baseDepartment: "engineering" (stored as custom attribute)
       - extraMembers: ["dave", "eve"] (stored as custom attribute)
       - repositories: ["new-project"]
    2. Controller resolves dynamic membership at sync time:
       - Fetch engineering.members → [alice, bob, charlie]
       - Merge with extraMembers → [alice, bob, charlie, dave, eve]
    3. SyncGroupToTeam("collab-new-project", ..., "write")
    4. Gitea: collab-new-project team with 5 members + new-project repo

Result:
    All of engineering + dave + eve can push/pull new-project
    New hires to engineering automatically get access (dynamic resolution)
```

### Remove access: Drop group off a repo

```
Frontend action:
    Click X on group chip attached to repo, or drag group out

API call:
    mutation { removeRepoFromGroup(groupCN: "backend-devs", repo: "api-gateway") }

Backend flow:
    1. LDAP Manager: remove "api-gateway" from backend-devs.repositories
    2. Immediate: SyncGroupToTeam("backend-devs", ...)
    3. Gitea: backend-devs team loses access to api-gateway
```

### Delete collab group entirely

```
Frontend action:
    Click delete on a collab-* group

API call:
    mutation { deleteCollabGroup(groupCN: "collab-new-project") }

Backend flow:
    1. LDAP Manager: delete group "collab-new-project"
    2. Gitea: delete team "collab-new-project"
    3. All members lose access to repos that were only accessible via this group
```

---

## UI Mockup

```
┌──────────────────────────────────────────────────────────────────┐
│  Repository: api-gateway                                         │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────────────────┐  ┌──────────────────────────────┐ │
│  │  Groups & Departments    │  │  Groups with access          │ │
│  │  (drag to add access)    │  │                              │ │
│  │                          │  │  ┌────────────────────────┐  │ │
│  │  ┌────────────────────┐  │  │  │ backend-devs    [x]   │  │ │
│  │  │ frontend-team      │──drag──│ 3 members, write       │  │ │
│  │  └────────────────────┘  │  │  └────────────────────────┘  │ │
│  │  ┌────────────────────┐  │  │  ┌────────────────────────┐  │ │
│  │  │ qa-team            │  │  │  │ engineering      [x]   │  │ │
│  │  └────────────────────┘  │  │  │ 4 members, write       │  │ │
│  │  ┌────────────────────┐  │  │  └────────────────────────┘  │ │
│  │  │ datascience        │  │  │                              │ │
│  │  └────────────────────┘  │  │  Drop group here to grant   │ │
│  │                          │  │  write access                │ │
│  │  Departments:            │  │                              │ │
│  │  ┌────────────────────┐  │  └──────────────────────────────┘ │
│  │  │ engineering    [+] │  │                                    │
│  │  └────────────────────┘  │  ┌──────────────────────────────┐ │
│  │  ┌────────────────────┐  │  │  Custom collab groups        │ │
│  │  │ devops         [+] │  │  │                              │ │
│  │  └────────────────────┘  │  │  ┌────────────────────────┐  │ │
│  │                          │  │  │ collab-api-review  [x] │  │ │
│  │  [+] = click to create  │  │  │ engineering + dave,eve  │  │ │
│  │  collab group with extra │  │  │ 6 members, write       │  │ │
│  │  members                 │  │  └────────────────────────┘  │ │
│  └──────────────────────────┘  └──────────────────────────────┘ │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────────┐│
│  │ All users with access (resolved from groups above):          ││
│  │ alice, bob, charlie, dave, eve, frank                        ││
│  └──────────────────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────────┘
```

---

## Dynamic Membership Resolution

Collab groups with a `baseDepartment` attribute are **dynamic**.
The controller resolves actual members at sync time:

```
Every sync cycle (5 minutes):
    For each LDAP group with baseDepartment attribute:
        1. Fetch baseDepartment members from LDAP
        2. Fetch extraMembers attribute from group
        3. Merge: finalMembers = departmentMembers + extraMembers
        4. SyncGroupToTeam with finalMembers

    For regular LDAP groups (no baseDepartment):
        1. Fetch group members directly from LDAP
        2. SyncGroupToTeam with those members
```

This means:
- New hire joins "engineering" department → next sync cycle they get access
  to all repos that have engineering or engineering-based collab groups
- Remove "dave" from extraMembers → next sync cycle dave loses access
- Delete the collab group → team deleted, all access revoked

---

## Data Model

### LDAP Group (regular)

```
cn=backend-devs,ou=groups,dc=devplatform,dc=local
    objectClass: groupOfNames, posixGroup, extensibleObject
    cn: backend-devs
    members: [uid=alice, uid=bob, uid=charlie]
    githubRepository: [api-gateway, auth-service]
```

### LDAP Group (collab with dynamic membership)

```
cn=collab-new-project,ou=groups,dc=devplatform,dc=local
    objectClass: groupOfNames, posixGroup, extensibleObject
    cn: collab-new-project
    description: "Collaboration group for new-project"
    baseDepartment: engineering              ← custom attribute via extensibleObject
    extraMembers: [dave, eve]                ← custom attribute via extensibleObject
    githubRepository: [new-project]
```

### LDAP Department (existing)

```
ou=engineering,ou=departments,dc=devplatform,dc=local
    objectClass: organizationalUnit, extensibleObject
    ou: engineering
    members: [uid=alice, uid=bob, uid=charlie, uid=frank]
    githubRepository: [shared-libs, infra-tools]
```

---

## GraphQL API

### Queries

```graphql
# List groups/departments that have access to a repo
repositoryGroups(owner: String!, repo: String!): [GroupAccess!]!

# List all collab groups (collab-* prefix)
collabGroups: [CollabGroup!]!

# Get resolved members of a group (including dynamic department resolution)
resolvedGroupMembers(groupCN: String!): [String!]!
```

### Mutations

```graphql
# Add a repo to a group's repository list + immediate sync
addRepoToGroup(groupCN: String!, repo: String!): SyncResult!

# Remove a repo from a group's repository list + immediate sync
removeRepoFromGroup(groupCN: String!, repo: String!): SyncResult!

# Add a repo to a department's repository list + immediate sync
addRepoToDepartment(ou: String!, repo: String!): SyncResult!

# Remove a repo from a department's repository list + immediate sync
removeRepoFromDepartment(ou: String!, repo: String!): SyncResult!

# Create a collab group with dynamic membership + immediate sync
createCollabGroup(
    name: String!
    baseDepartment: String!
    extraMembers: [String!]!
    repositories: [String!]!
): CollabGroup!

# Add extra members to an existing collab group
addCollabGroupMembers(groupCN: String!, members: [String!]!): CollabGroup!

# Remove extra members from a collab group
removeCollabGroupMembers(groupCN: String!, members: [String!]!): CollabGroup!

# Add repos to a collab group
addCollabGroupRepos(groupCN: String!, repos: [String!]!): CollabGroup!

# Delete a collab group and its Gitea team
deleteCollabGroup(groupCN: String!): Boolean!
```

### Types

```graphql
type GroupAccess {
    groupCN: String!
    groupType: String!          # "group", "department", "collab"
    members: [String!]!         # resolved member list
    permission: String!         # "write"
    baseDepartment: String      # only for collab groups
    extraMembers: [String!]     # only for collab groups
}

type CollabGroup {
    cn: String!
    description: String
    baseDepartment: String!
    extraMembers: [String!]!
    repositories: [String!]!
    resolvedMembers: [String!]! # department members + extra members
}

type SyncResult {
    team: Team
    membersAdded: Int!
    membersFailed: Int!
    repositoriesAdded: Int!
    repositoriesFailed: Int!
    errors: [String!]
}
```

---

## Controller Changes

### 4th Goroutine: Group Sync Loop

```go
// groupSyncLoop periodically syncs all LDAP groups to Gitea teams
func (c *Controller) groupSyncLoop() {
    defer c.wg.Done()

    // Initial delay
    select {
    case <-time.After(20 * time.Second):
    case <-c.stopCh:
        return
    }

    c.runGroupSync()

    ticker := time.NewTicker(c.cfg.GroupSyncInterval) // default 5m
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            c.runGroupSync()
        case <-c.stopCh:
            return
        }
    }
}

// runGroupSync fetches all LDAP groups and syncs each to a Gitea team
func (c *Controller) runGroupSync() {
    token := c.getKeycloakToken()
    groups := c.ldapClient.ListGroups(ctx, token)

    for _, group := range groups {
        members := group.Members

        // Dynamic membership resolution for collab groups
        if group.BaseDepartment != "" {
            dept := c.ldapClient.GetDepartment(ctx, group.BaseDepartment, token)
            members = mergeMembersUnique(dept.Members, group.ExtraMembers)
        }

        c.groupSyncService.SyncGroupToTeam(ctx, group.CN, orgName, group.CN, "write")
    }
}
```

### Immediate Sync on Mutation

When the GraphQL API adds/removes a repo from a group, it calls
`SyncGroupToTeam` immediately (not waiting for the next cycle).
The periodic sync is a safety net to catch drift.

---

## Access Resolution

When checking if a user can access a repo:

```
1. Is the user the repo owner (gitea_admin)?     → full access
2. Is the user in a Gitea team that has the repo? → team permission (write)
3. None of the above                              → denied
```

The teams come from:
- Regular LDAP groups synced to Gitea teams
- Departments synced to Gitea teams
- Collab groups (dynamic) synced to Gitea teams

All use `write` permission (full push/pull/merge access).

---

## Implementation Files

| File | Action | What |
|------|--------|------|
| `internal/ldap/client.go` | MODIFY | Add `ListGroups`, `CreateCollabGroup`, `DeleteCollabGroup`, `AddRepoToGroup`, `RemoveRepoFromGroup`, `GetCollabGroup`, `UpdateCollabGroupMembers` |
| `internal/sync/group_sync.go` | MODIFY | Add dynamic membership resolution (baseDepartment + extraMembers merge) |
| `internal/sync/controller.go` | MODIFY | Add 4th goroutine `groupSyncLoop` + `runGroupSync` |
| `internal/gitea/service.go` | MODIFY | Add `AddRepoToGroupAndSync`, `RemoveRepoFromGroupAndSync`, `CreateCollabGroupAndSync`, `DeleteCollabGroupAndSync` |
| `internal/graphql/schema.go` | MODIFY | Add collab queries and mutations |
| `internal/graphql/types_collab.go` | NEW | GraphQL types: `GroupAccess`, `CollabGroup` |
| `internal/config/config.go` | MODIFY | Add `GroupSyncInterval` config field |
| `k8s/01-configmap.yaml` | MODIFY | Add `GROUP_SYNC_INTERVAL` |

---

## Existing Code Reused

| Existing code | Used for |
|---------------|----------|
| `sync/group_sync.go → SyncGroupToTeam` | Core sync: LDAP group → Gitea team |
| `sync/group_sync.go → SyncMultipleGroups` | Batch sync all groups in one call |
| `sync/group_sync.go → RemoveMembersNotInLDAP` | Clean up stale team members |
| `gitea/teams.go → CreateTeam, AddTeamMember, AddTeamRepository` | Gitea team CRUD |
| `ldap/client.go → GetGroup` | Fetch LDAP group data |
| `ldap/client.go → AssignReposToUser` | Pattern for LDAP attribute updates |
| `sync/controller.go → reconcileLoop pattern` | Same goroutine pattern for groupSyncLoop |
| `sync/controller.go → getKeycloakToken` | Auth for controller-initiated syncs |
