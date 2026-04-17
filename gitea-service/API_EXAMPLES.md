# Gitea Service API Examples

Complete examples for all git operations with auto-default owner support.

## Authentication

All requests require a JWT token from the LDAP Manager:

```bash
# Get token from LDAP Manager
curl -X POST http://ldap-manager/graphql \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation { login(uid: \"john.doe\", password: \"password123\") { token } }"
  }'

# Use token in Gitea Service
curl -X POST http://gitea-service/graphql \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{ "query": "..." }'
```

---

## Repository Operations

### Create Repository (Simple - Auto Default Owner)

```graphql
mutation {
  createRepository(
    name: "my-awesome-project"
    description: "My new project"
    private: false
    autoInit: true
  ) {
    id
    name
    fullName        # "admin123/my-awesome-project"
    cloneUrl
    sshUrl
    htmlUrl
    private
    defaultBranch
    createdAt
  }
}
```

### Create Repository (With All Options)

```graphql
mutation {
  createRepository(
    name: "go-microservice"
    description: "Production microservice in Go"
    private: true
    autoInit: true
    gitignores: "Go"                    # Add .gitignore
    license: "MIT"                       # Add MIT license
    defaultBranch: "main"                # Set default branch
  ) {
    id
    name
    fullName
    cloneUrl
    defaultBranch
  }
}
```

### Migrate Repository from GitHub (Simple)

```graphql
mutation {
  migrateRepository(
    cloneAddr: "https://github.com/facebook/react"
    repoName: "react-mirror"
    service: "github"
  ) {
    id
    name
    fullName       # "admin123/react-mirror"
    cloneUrl
    stars
    forks
  }
}
```

### Migrate Private Repository from GitHub

```graphql
mutation {
  migrateRepository(
    cloneAddr: "https://github.com/myorg/private-backend"
    repoName: "backend-api"
    service: "github"
    private: true
    authToken: "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
    wiki: true
    issues: true
    pullRequests: true
    releases: true
    milestones: true
    labels: true
  ) {
    id
    name
    fullName
    private
    cloneUrl
  }
}
```

### Migrate from GitLab

```graphql
mutation {
  migrateRepository(
    cloneAddr: "https://gitlab.com/gitlab-org/gitlab"
    repoName: "gitlab-mirror"
    service: "gitlab"
    authToken: "glpat-xxxxxxxxxxxxxxxxxxxx"
  ) {
    id
    name
    fullName
  }
}
```

### Create Mirror Repository

```graphql
mutation {
  migrateRepository(
    cloneAddr: "https://github.com/kubernetes/kubernetes"
    repoName: "k8s-mirror"
    service: "github"
    mirror: true           # This creates a mirror that auto-syncs
    wiki: false
    issues: false
    pullRequests: false
  ) {
    id
    name
    fullName
  }
}
```

### Fork Repository

```graphql
mutation {
  forkRepository(
    owner: "admin123"
    repo: "existing-repo"
    organization: ""       # Leave empty to fork to your user
  ) {
    id
    name
    fullName
    fork
    cloneUrl
  }
}
```

### List My Repositories

```graphql
query {
  myRepositories(limit: 20, offset: 0) {
    items {
      id
      name
      fullName
      description
      private
      fork
      language
      stars
      forks
      size
      defaultBranch
      cloneUrl
      sshUrl
      htmlUrl
      createdAt
      updatedAt
      owner {
        login
        fullName
        email
      }
    }
    total
    limit
    offset
    hasMore
  }
}
```

### Search Repositories

```graphql
query {
  searchRepositories(
    query: "backend"
    limit: 10
    offset: 0
  ) {
    items {
      name
      fullName
      description
      language
      stars
    }
    total
    hasMore
  }
}
```

### Get Repository Details

```graphql
query {
  getRepository(owner: "admin123", name: "my-repo") {
    id
    name
    fullName
    description
    private
    fork
    stars
    forks
    language
    size
    defaultBranch
    cloneUrl
    sshUrl
    htmlUrl
    createdAt
    updatedAt
    owner {
      login
      fullName
      avatarUrl
    }
  }
}
```

### Update Repository

```graphql
mutation {
  updateRepository(
    owner: "admin123"
    name: "my-repo"
    description: "Updated description"
    private: true
    defaultBranch: "main"
  ) {
    id
    name
    description
    private
    defaultBranch
  }
}
```

### Delete Repository

```graphql
mutation {
  deleteRepository(
    owner: "admin123"
    name: "old-repo"
  )
}
```

---

## Branch Operations

### List Branches

```graphql
query {
  listBranches(owner: "admin123", repo: "my-repo") {
    name
    commit {
      sha
      url
      created
    }
    protected
    requiredApprovals
  }
}
```

### Get Branch

```graphql
query {
  getBranch(
    owner: "admin123"
    repo: "my-repo"
    branch: "main"
  ) {
    name
    commit {
      sha
      url
    }
    protected
  }
}
```

### Create Branch

```graphql
mutation {
  createBranch(
    owner: "admin123"
    repo: "my-repo"
    branchName: "feature/new-feature"
    oldBranchName: "main"
  ) {
    name
    commit {
      sha
      url
    }
  }
}
```

### Delete Branch

```graphql
mutation {
  deleteBranch(
    owner: "admin123"
    repo: "my-repo"
    branch: "feature/old-feature"
  )
}
```

---

## Commit Operations

### List Commits

```graphql
query {
  listCommits(
    owner: "admin123"
    repo: "my-repo"
  ) {
    sha
    url
    commit {
      message
      author {
        name
        email
        date
      }
      committer {
        name
        email
        date
      }
    }
  }
}
```

### List Commits on Branch

```graphql
query {
  listCommits(
    owner: "admin123"
    repo: "my-repo"
    sha: "develop"         # Branch name or commit SHA
  ) {
    sha
    commit {
      message
      author {
        name
        email
      }
    }
  }
}
```

### List Commits for File

```graphql
query {
  listCommits(
    owner: "admin123"
    repo: "my-repo"
    path: "src/main.go"    # Only commits affecting this file
  ) {
    sha
    commit {
      message
      author {
        name
        email
      }
    }
  }
}
```

### Get Commit Details

```graphql
query {
  getCommit(
    owner: "admin123"
    repo: "my-repo"
    sha: "a1b2c3d4e5f6"
  ) {
    sha
    url
    commit {
      message
      tree {
        sha
        url
      }
      author {
        name
        email
        date
      }
      committer {
        name
        email
        date
      }
    }
    author {
      name
      email
    }
    committer {
      name
      email
    }
  }
}
```

---

## Tag Operations

### List Tags

```graphql
query {
  listTags(owner: "admin123", repo: "my-repo") {
    name
    message
    commit {
      sha
      url
      created
    }
    zipballUrl
    tarballUrl
  }
}
```

### Create Tag

```graphql
mutation {
  createTag(
    owner: "admin123"
    repo: "my-repo"
    tagName: "v1.0.0"
    target: "main"                    # Branch or commit SHA
    message: "Release version 1.0.0"
  ) {
    name
    message
    commit {
      sha
      url
    }
    zipballUrl
    tarballUrl
  }
}
```

### Create Lightweight Tag (No Message)

```graphql
mutation {
  createTag(
    owner: "admin123"
    repo: "my-repo"
    tagName: "v1.0.1"
    target: "a1b2c3d4e5f6"     # Specific commit
  ) {
    name
    commit {
      sha
    }
  }
}
```

### Delete Tag

```graphql
mutation {
  deleteTag(
    owner: "admin123"
    repo: "my-repo"
    tag: "v0.9.0"
  )
}
```

---

## Statistics

### Repository Stats

```graphql
query {
  repositoryStats {
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

---

## Health Check

### Check Service Health

```graphql
query {
  health {
    status          # "healthy" or "unhealthy"
    timestamp
    gitea           # true if Gitea is reachable
    ldapManager     # true if LDAP Manager is reachable
  }
}
```

---

## Complete Workflow Examples

### Example 1: Fork, Branch, and Tag

```graphql
# 1. Fork a repository
mutation {
  forkRepository(
    owner: "admin123"
    repo: "main-project"
  ) {
    name
    fullName
  }
}

# 2. Create a feature branch
mutation {
  createBranch(
    owner: "admin123"
    repo: "main-project"
    branchName: "feature/user-auth"
    oldBranchName: "main"
  ) {
    name
  }
}

# 3. After work is done, create a release tag
mutation {
  createTag(
    owner: "admin123"
    repo: "main-project"
    tagName: "v1.1.0"
    target: "feature/user-auth"
    message: "Added user authentication"
  ) {
    name
    zipballUrl
  }
}
```

### Example 2: Migrate Multiple GitHub Repos

```graphql
# Migrate frontend
mutation {
  migrateFrontend: migrateRepository(
    cloneAddr: "https://github.com/myorg/frontend"
    repoName: "team-frontend"
    service: "github"
    authToken: "ghp_xxxxx"
    private: true
  ) {
    name
  }
}

# Migrate backend
mutation {
  migrateBackend: migrateRepository(
    cloneAddr: "https://github.com/myorg/backend"
    repoName: "team-backend"
    service: "github"
    authToken: "ghp_xxxxx"
    private: true
  ) {
    name
  }
}
```

### Example 3: Full Repository Setup

```graphql
mutation {
  createRepository(
    name: "new-microservice"
    description: "User authentication microservice"
    private: true
    autoInit: true
    gitignores: "Go"
    license: "MIT"
    defaultBranch: "main"
  ) {
    id
    name
    cloneUrl
  }
}

# Then create development branch
mutation {
  createBranch(
    owner: "admin123"
    repo: "new-microservice"
    branchName: "develop"
    oldBranchName: "main"
  ) {
    name
  }
}

# Create initial tag
mutation {
  createTag(
    owner: "admin123"
    repo: "new-microservice"
    tagName: "v0.1.0"
    target: "main"
    message: "Initial setup"
  ) {
    name
  }
}
```

---

## cURL Examples

### Create Repository

```bash
curl -X POST http://gitea-service:8081/graphql \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation { createRepository(name: \"test-repo\", autoInit: true) { id name cloneUrl } }"
  }'
```

### Migrate from GitHub

```bash
curl -X POST http://gitea-service:8081/graphql \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation { migrateRepository(cloneAddr: \"https://github.com/user/repo\", repoName: \"migrated-repo\", service: \"github\") { id name cloneUrl } }"
  }'
```

---

## Notes

1. **Auto-default owner**: When `owner` is not specified, it defaults to `GITEA_DEFAULT_OWNER` (usually `admin123`)

2. **Authentication**: All operations require a valid JWT token from LDAP Manager

3. **Access Control**: Access is checked against LDAP `githubRepository` attributes

4. **Repository naming**:
   - In LDAP: Can be just name (`myrepo`) or full (`owner/myrepo`)
   - In Gitea: Always `owner/name` format

5. **Migration services**: Supported values for `service`:
   - `"github"` - GitHub
   - `"gitlab"` - GitLab
   - `"gitea"` - Another Gitea instance
   - `"gogs"` - Gogs

6. **Private repo migration**: Always provide `authToken` or `authUsername`/`authPassword` for private repos
