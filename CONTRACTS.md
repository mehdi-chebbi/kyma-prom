# Cross-Service API Contracts

> **Single source of truth** for all inter-service communication.
> Every service change that affects a shared type, query, or mutation **MUST** be reflected here first.
> This prevents field-name drift, missing-field errors, and silent fallbacks.

---

## Service Topology

```
Frontend (React/TS)
  |
  |-- VITE_GRAPHQL_ENDPOINT ---------> LDAP Manager       (port 8080)
  |-- VITE_GITEA_API_ENDPOINT -------> Gitea Service      (port 8081)
  |-- VITE_CODESERVER_API_ENDPOINT --> CodeServer Service  (port 8082)

Gitea Service (Go)
  |-- calls LDAP Manager via HTTP/GraphQL  (LDAP_MANAGER_URL)
  |-- calls Gitea Server via REST API      (GITEA_URL)

CodeServer Service (Go)
  |-- calls Gitea Service via HTTP/GraphQL (GITEA_SERVICE_URL)
  |-- calls Kubernetes API                 (in-cluster)
```

---

## 1. LDAP Manager (backend/)

### Types

#### User
| Field | Type | Notes |
|-------|------|-------|
| uid | String | Username |
| cn | String | Common name (full name) |
| sn | String | Surname |
| givenName | String | First name |
| mail | String | Email |
| department | String | Department name |
| repositories | [String] | LDAP `githubRepository` multi-value |
| dn | String | Full LDAP DN |

> **WARNING**: `uidNumber`, `gidNumber`, `homeDirectory` are stored in LDAP but are **NOT** exposed in the GraphQL schema. Do not query them.

#### Department
| Field | Type | Notes |
|-------|------|-------|
| ou | String | OU name |
| description | String | |
| manager | String | Manager DN |
| members | [String] | User UIDs in this department |
| repositories | [String] | LDAP `githubRepository` multi-value |
| dn | String | Full LDAP DN |

#### Group
| Field | Type | Notes |
|-------|------|-------|
| cn | String | Group name |
| description | String | |
| gidNumber | Int | POSIX GID |
| members | [String] | Member DNs |
| repositories | [String] | LDAP `githubRepository` multi-value |
| dn | String | Full LDAP DN |

#### Stats
| Field | Type | Notes |
|-------|------|-------|
| totalConnections | Int | |
| activeConnections | Int | |
| poolSize | Int | |

#### Health
| Field | Type | Notes |
|-------|------|-------|
| status | String | |
| timestamp | String | |

### Queries

| Query | Args | Returns |
|-------|------|---------|
| `me` | — | User |
| `user` | uid: String! | User |
| `users` | filter: SearchFilterInput, limit: Int=10, offset: Int=0 | PaginatedUsers |
| `usersAll` | — | [User] |
| `department` | ou: String! | Department |
| `departments` | filter: DepartmentFilterInput, limit: Int=10, offset: Int=0 | PaginatedDepartments |
| `departmentsAll` | — | [Department] |
| `departmentUsers` | department: String!, limit: Int=10, offset: Int=0 | PaginatedUsers |
| `group` | cn: String! | Group |
| `groups` | filter: GroupFilterInput, limit: Int=10, offset: Int=0 | PaginatedGroups |
| `groupsAll` | — | [Group] |
| `health` | — | Health |
| `stats` | — | Stats |

### Mutations

| Mutation | Args | Returns |
|----------|------|---------|
| `createUser` | input: CreateUserInput! | User |
| `updateUser` | input: UpdateUserInput! | User |
| `deleteUser` | uid: String! | Boolean |
| `createDepartment` | input: CreateDepartmentInput! | Department |
| `deleteDepartment` | ou: String! | Boolean |
| `assignRepoToDepartment` | ou: String!, repositories: [String!]! | Department |
| `assignRepoToUser` | uid: String!, repositories: [String!]! | User |
| `assignRepoToGroup` | groupCn: String!, repositories: [String!]! | Group |
| `createGroup` | cn: String!, description: String | Group |
| `addUserToGroup` | uid: String!, groupCn: String! | Boolean |
| `removeUserFromGroup` | uid: String!, groupCn: String! | Boolean |
| `deleteGroup` | cn: String! | Boolean |

### Input Types

**SearchFilterInput**: uid, cn, sn, givenName, mail, department, uidNumber (Int), gidNumber (Int), repository
**DepartmentFilterInput**: ou, description
**GroupFilterInput**: cn
**CreateUserInput**: uid!, cn!, givenName, sn!, mail!, password!, department, repositories
**UpdateUserInput**: uid!, cn, givenName, sn, mail, password, department, repositories
**CreateDepartmentInput**: ou!, description, manager, repositories

---

## 2. Gitea Service (gitea-service/)

### Types

#### Repository
| Field | Type | Notes |
|-------|------|-------|
| id | Int | |
| owner | RepositoryOwner | Nested object |
| name | String | |
| fullName | String | `owner/name` |
| description | String | |
| private | Boolean | |
| fork | Boolean | |
| htmlUrl | String | |
| sshUrl | String | |
| cloneUrl | String | |
| defaultBranch | String | |
| createdAt | String | |
| updatedAt | String | |
| language | String | |
| size | Int | |
| stars | Int | **Not** `starsCount` |
| forks | Int | **Not** `forksCount` |
| openIssuesCount | Int | |
| archived | Boolean | |

#### RepositoryOwner
| Field | Type |
|-------|------|
| id | Int |
| login | String |
| fullName | String |
| email | String |
| avatarUrl | String |

#### PaginatedRepositories
| Field | Type |
|-------|------|
| items | [Repository] |
| total | Int |
| limit | Int |
| offset | Int |
| hasMore | Boolean |

#### RepositoryStats
| Field | Type | Notes |
|-------|------|-------|
| total | Int | Backend field name |
| public | Int | Backend field name |
| private | Int | Backend field name |

> **Frontend queries as**: `totalCount`, `publicCount`, `privateCount`, `languages { language, count }` — verify these match the backend schema.

#### Branch
| Field | Type |
|-------|------|
| name | String |
| commit | { id: String, url: String } |

#### Commit
| Field | Type |
|-------|------|
| sha | String |
| commit | { message, author: { name, email, date } } |
| htmlUrl | String |

#### Tag
| Field | Type |
|-------|------|
| name | String |
| message | String |
| commit | { sha, url } |
| zipballUrl | String |
| tarballUrl | String |

#### PullRequest
| Field | Type |
|-------|------|
| id | Int |
| number | Int |
| state | String |
| title | String |
| body | String |
| user | PRUser |
| head | PRBranchInfo { label, ref, sha } |
| base | PRBranchInfo { label, ref, sha } |
| mergeable | Boolean |
| merged | Boolean |
| mergedAt | String |
| mergedBy | PRUser |
| createdAt | String |
| updatedAt | String |
| closedAt | String |
| dueDate | String |
| assignees | [PRUser] |
| labels | [PRLabel { id, name, color }] |
| milestone | PRMilestone { id, title, description, state, dueOn } |
| comments | Int |
| additions | Int |
| deletions | Int |
| changedFiles | Int |
| htmlUrl | String |
| diffUrl | String |
| patchUrl | String |

#### PRUser
`{ id: Int, login: String, fullName: String, email: String, avatarUrl: String }`

#### GiteaUser
`{ id: Int, login: String, fullName: String, email: String, avatarUrl: String, isAdmin: Boolean, created: String }`

#### Issue
`{ id, number, title, body, state, user: IssueUser, labels: [IssueLabel], milestone: IssueMilestone, createdAt, updatedAt }`

#### RepoSyncResult
`{ uid: String, reposCount: Int, repositories: [String] }`

#### Team
`{ id: Int, name: String, description: String, permission: TeamPermission(READ|WRITE|ADMIN), members: [GiteaUser], repositories: [Repository] }`

#### SyncResult
`{ team: Team, membersAdded: Int, membersFailed: Int, repositoriesAdded: Int, repositoriesFailed: Int, errors: [String] }`

#### GroupAccess
`{ cn, groupType, members, permission, baseDepartment, extraMembers }`

### Queries

| Query | Args | Returns |
|-------|------|---------|
| `listRepositories` | limit: Int=10, offset: Int=0 | PaginatedRepositories |
| `searchRepositories` | query: String, limit, offset | PaginatedRepositories |
| `getRepository` | owner: String!, name: String! | Repository |
| `myRepositories` | limit: Int=10, offset: Int=0 | PaginatedRepositories |
| `repositoryStats` | — | RepositoryStats |
| `health` | — | Health |
| `listBranches` | owner!, repo!, page=1, limit=100 | [Branch] |
| `getBranch` | owner!, repo!, branch! | Branch |
| `listCommits` | owner!, repo!, sha?, path?, page=1, limit=50 | [Commit] |
| `getCommit` | owner!, repo!, sha! | Commit |
| `listTags` | owner!, repo!, page=1, limit=50 | [Tag] |
| `listPullRequests` | owner!, repo!, state="open", page=1, limit=30 | [PullRequest] |
| `getPullRequest` | owner!, repo!, number: Int! | PullRequest |
| `listPRComments` | owner!, repo!, number!, page=1, limit=30 | [PRComment] |
| `listPRReviews` | owner!, repo!, number!, page=1, limit=30 | [PRReview] |
| `listPRFiles` | owner!, repo!, number!, page=1, limit=100 | [PRFile] |
| `getPRDiff` | owner!, repo!, number! | String |
| `isPRMerged` | owner!, repo!, number! | Boolean |
| `giteaUser` | username: String! | GiteaUser |
| `searchGiteaUsers` | query: String!, limit=10 | [GiteaUser] |
| `listIssues` | owner?, repo!, state="open", labels?, page=1, limit=30 | [Issue] |
| `getIssue` | owner?, repo!, number! | Issue |
| `listIssueComments` | owner?, repo!, number! | [IssueComment] |
| `listLabels` | owner?, repo!, page=1, limit=30 | [IssueLabel] |
| `listMilestones` | owner?, repo!, state="open", page=1, limit=30 | [IssueMilestone] |
| `listTeams` | orgName!, page=1, limit=50 | [Team] |
| `getTeam` | teamId: Int! | Team |
| `listRepoAccess` | repo: String! | [GroupAccess] |

### Mutations

| Mutation | Args | Returns |
|----------|------|---------|
| `createRepository` | owner?, name!, description?, private=false, autoInit=true, gitignores?, license?, readme?, defaultBranch? | Repository |
| `updateRepository` | owner!, name!, description?, private?, defaultBranch? | Repository |
| `deleteRepository` | owner!, name! | Boolean |
| `migrateRepository` | cloneAddr!, repoName!, repoOwner?, mirror=false, private=false, description?, wiki=true, milestones=true, labels=true, issues=true, pullRequests=true, releases=true, authUsername?, authPassword?, authToken?, service? | Repository |
| `forkRepository` | owner!, repo!, organization? | Repository |
| `createBranch` | owner!, repo!, branchName!, oldBranchName! | Branch |
| `deleteBranch` | owner!, repo!, branch! | Boolean |
| `createTag` | owner!, repo!, tagName!, target!, message? | Tag |
| `deleteTag` | owner!, repo!, tag! | Boolean |
| `createPullRequest` | owner!, repo!, title!, body?, head!, base! | PullRequest |
| `updatePullRequest` | owner!, repo!, number!, title?, body?, state? | PullRequest |
| `mergePullRequest` | owner!, repo!, number!, method="merge", deleteBranchAfterMerge=false | Boolean |
| `createPRComment` | owner!, repo!, number!, body! | PRComment |
| `createPRReview` | owner!, repo!, number!, event!, body? | PRReview |
| `syncLDAPUser` | uid!, defaultPassword="changeme123" | GiteaUser |
| `syncAllLDAPUsers` | defaultPassword="changeme123" | [GiteaUser] |
| `syncGiteaReposToLDAP` | uid! | RepoSyncResult |
| `syncAllGiteaReposToLDAP` | — | [RepoSyncResult] |
| `createIssue` | owner?, repo!, title!, body?, assignees?, labels?, milestone? | Issue |
| `updateIssue` | owner?, repo!, number!, title?, body?, state?, assignees?, labels?, milestone? | Issue |
| `createIssueComment` | owner?, repo!, number!, body! | IssueComment |
| `createLabel` | owner?, repo!, name!, color!, description? | IssueLabel |
| `createMilestone` | owner?, repo!, title!, description?, state="open" | IssueMilestone |
| `createTeam` | orgName!, name!, description?, permission! | Team |
| `addTeamMember` | teamId!, username! | Team |
| `removeTeamMember` | teamId!, username! | Team |
| `addTeamRepository` | teamId!, owner!, repo! | Team |
| `removeTeamRepository` | teamId!, owner!, repo! | Team |
| `addRepoToGroup` | groupCn!, repo! | SyncResult |
| `removeRepoFromGroup` | groupCn!, repo! | SyncResult |
| `addRepoToDepartment` | ou!, repo! | SyncResult |
| `removeRepoFromDepartment` | ou!, repo! | SyncResult |
| `createCollabGroup` | name!, baseDepartment!, extraMembers?, repos? | SyncResult |
| `deleteCollabGroup` | groupCn! | Boolean |
| `syncGroupToTeam` | groupCn!, orgName!, teamName?, permission! | SyncResult |

---

## 3. CodeServer Service (codeserver-service/)

### Types

#### CodeServerInstance
| Field | Type | Notes |
|-------|------|-------|
| id | String! | |
| userId | String! | |
| repoName | String! | |
| repoOwner | String! | |
| url | String! | |
| status | InstanceStatus! | PENDING, STARTING, RUNNING, STOPPING, STOPPED, ERROR |
| createdAt | String! | |
| lastAccessedAt | String | nullable |
| storageUsed | String | nullable |
| errorMessage | String | nullable |

#### ProvisionResult
| Field | Type |
|-------|------|
| instance | CodeServerInstance! |
| message | String! |
| isNew | Boolean! |

#### InstanceStats
| Field | Type |
|-------|------|
| totalInstances | Int! |
| runningInstances | Int! |
| stoppedInstances | Int! |
| pendingInstances | Int! |
| totalStorageUsed | String! |

### Queries

| Query | Args | Returns |
|-------|------|---------|
| `health` | — | Boolean! |
| `myCodeServers` | — | [CodeServerInstance!]! |
| `codeServer` | id: String! | CodeServerInstance |
| `codeServerStatus` | id: String! | InstanceStatus! |
| `codeServerLogs` | id: String!, lines: Int=100 | String! |
| `instanceStats` | — | InstanceStats! |
| `myRepositories` | — | [Repository!]! |

### Mutations

| Mutation | Args | Returns |
|----------|------|---------|
| `provisionCodeServer` | repoOwner!, repoName!, branch? | ProvisionResult! |
| `stopCodeServer` | id! | Boolean! |
| `startCodeServer` | id! | CodeServerInstance |
| `deleteCodeServer` | id! | Boolean! |
| `syncRepository` | id! | Boolean! |

---

## 4. Cross-Service Calls

### gitea-service --> LDAP Manager

Client: `gitea-service/internal/ldap/client.go`
Auth: Forwards user JWT as `Authorization: Bearer {token}`

| Method | LDAP Query/Mutation | Fields Requested |
|--------|--------------------|--------------------|
| `GetUser` | `user(uid)` | uid, cn, sn, givenName, mail, department, repositories, dn |
| `GetDepartment` | `department(ou)` | ou, description, manager, members, repositories, dn |
| `ListAllUsers` | `usersAll` | uid, cn, mail, department, repositories |
| `ListAllGroups` | `groupsAll` | cn, description, gidNumber, members, repositories, dn |
| `ListAllDepartments` | `departmentsAll` | ou, description, manager, members, repositories, dn |
| `GetGroup` | `group(cn)` | cn, gidNumber, description, members, repositories, dn |
| `AssignReposToUser` | `assignRepoToUser` | uid, repositories |
| `AssignReposToGroup` | `assignRepoToGroup` | cn, repositories |
| `AssignReposToDepartment` | `assignRepoToDepartment` | ou, repositories |
| `CreateGroup` | `createGroup` | cn, description, gidNumber, members, repositories, dn |
| `DeleteGroup` | `deleteGroup` | Boolean |
| `AddUserToGroup` | `addUserToGroup` | Boolean |
| `RemoveUserFromGroup` | `removeUserFromGroup` | Boolean |
| `HealthCheck` | `GET /health` | HTTP status |

### codeserver-service --> gitea-service

Client: `codeserver-service/internal/gitea/client.go`
Auth: Forwards user JWT as `Authorization: Bearer {token}`

| Method | Gitea Query | Fields Requested |
|--------|-------------|--------------------|
| `GetUserRepositories` | `myRepositories` | id, name, fullName, owner.login, cloneUrl, sshUrl, htmlUrl, private, defaultBranch |
| `GetRepository` | `getRepository(owner, name)` | same as above |
| `ValidateRepoAccess` | calls `GetRepository` | checks non-nil |
| `GetRepoCloneURL` | calls `GetRepository` | returns cloneUrl |
| `HealthCheck` | `GET /health` | HTTP status |

---

## 5. Frontend Service Layer

### Routing

| Env Variable | Target | Used By |
|--------------|--------|---------|
| `VITE_GRAPHQL_ENDPOINT` | LDAP Manager | userService, departmentService, groupService, graphqlService |
| `VITE_GITEA_API_ENDPOINT` | Gitea Service | repositoryService (via `useGitea: true`) |
| `VITE_CODESERVER_API_ENDPOINT` | CodeServer Service | codeserverService (via `codeserverRequest`) |

### Frontend TypeScript Types

Located in `src/GQL/models/`:

**User** (`user.ts`): uid, cn, sn, givenName, mail, department, uidNumber, gidNumber, homeDirectory, repositories, dn
**Department** (`department.ts`): ou, description?, manager?, members, repositories, dn
**Group** (`group.ts`): cn, gidNumber, members, repositories?, dn
**Repository** (`repository.ts`): id, name, fullName, description?, private, fork, stars, forks, language?, size, cloneUrl, sshUrl, htmlUrl, defaultBranch, createdAt, updatedAt, owner
**CodeServerInstance** (inline in `codeserverService.ts`): id, userId, repoName, repoOwner, url, status, createdAt, lastAccessedAt?, storageUsed?, errorMessage?

---

## 6. Known Mismatches (Action Items)

### CRITICAL - Will cause runtime errors

| # | Issue | Location | Fix |
|---|-------|----------|-----|
| 1 | Frontend `User` type has `uidNumber`, `gidNumber`, `homeDirectory` but LDAP Manager does NOT expose them in GraphQL | `src/GQL/models/user.ts` | Remove these 3 fields from the TS type. They exist in LDAP but not in the API. |

### WARNING - Silent nulls / dead code

| # | Issue | Location | Fix |
|---|-------|----------|-----|
| 2 | Frontend requests `health { ldap }` but backend Health type only has `status`, `timestamp` | `src/services/graphqlService.ts` | Remove `ldap` from query or add it to backend schema |
| 3 | Frontend `StatsQuery` expects `poolSize, available, inUse, totalRequests` but backend Stats has `totalConnections, activeConnections, poolSize` | `src/GQL/apis/apis.ts` | Align field names |
| 4 | Frontend `RepositoryStats` uses `totalCount, publicCount, privateCount` but backend uses `total, public, private` | `src/services/repositoryService.ts` vs gitea-service schema | Verify — one side needs to change |
| 5 | `register` mutation referenced in frontend but doesn't exist in any backend | `src/GQL/apis/apis.ts` | Remove dead code |
| 6 | Frontend `UserFilter.query` and `DepartmentFilter.manager/repository` don't exist in backend filter inputs | `src/GQL/models/` | OK if only used for client-side filtering, but document clearly |

---

## 7. Rules for Updating Contracts

1. **Backend field rename** (e.g., `starsCount` -> `stars`): Update this doc, update all cross-service clients, update frontend service files and TS types
2. **New field on a type**: Add to this doc, add to any client that needs it
3. **New query/mutation**: Add to this doc with full signature
4. **Removing a field**: Search this doc for all consumers, update them first, then remove
5. **New service**: Add a new section following the same format

### Checklist for any schema change

- [ ] Updated `CONTRACTS.md` (this file)
- [ ] Updated backend GraphQL schema
- [ ] Updated cross-service clients (if service-to-service)
- [ ] Updated frontend service files (`src/services/*.ts`)
- [ ] Updated frontend TS types (`src/GQL/models/*.ts`)
- [ ] Verified `npm run dev` compiles
- [ ] Tested with a real query
