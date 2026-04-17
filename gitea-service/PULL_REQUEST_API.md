# Pull Request API Documentation

Complete guide for using the Pull Request functionality in the Gitea Service.

## Overview

The Pull Request API provides comprehensive functionality for code review workflows:

- Create, update, and merge pull requests
- Add comments and reviews
- View changed files and diffs
- Check merge status
- Full Git workflow support

All operations require JWT authentication and respect LDAP-based access control.

---

## Pull Request Operations

### List Pull Requests

Get all pull requests in a repository with filtering.

```graphql
query {
  listPullRequests(
    owner: "admin123"
    repo: "my-repo"
    state: "open"        # open, closed, all
    page: 1
    limit: 30
  ) {
    id
    number
    state
    title
    body
    user {
      login
      fullName
      avatarUrl
    }
    head {
      ref
      sha
    }
    base {
      ref
      sha
    }
    mergeable
    merged
    comments
    additions
    deletions
    changedFiles
    createdAt
    updatedAt
  }
}
```

**States:**
- `open` - Only open PRs (default)
- `closed` - Only closed PRs
- `all` - All PRs

### Get Pull Request Details

Retrieve detailed information about a specific pull request.

```graphql
query {
  getPullRequest(
    owner: "admin123"
    repo: "my-repo"
    number: 42
  ) {
    id
    number
    state
    title
    body
    user {
      login
      fullName
      email
      avatarUrl
    }
    head {
      label
      ref
      sha
    }
    base {
      label
      ref
      sha
    }
    mergeable
    merged
    mergedAt
    mergedBy {
      login
      fullName
    }
    closedAt
    dueDate
    assignees {
      login
      fullName
    }
    labels {
      name
      color
    }
    milestone {
      title
      state
      dueOn
    }
    comments
    additions
    deletions
    changedFiles
    htmlUrl
    diffUrl
    patchUrl
    createdAt
    updatedAt
  }
}
```

### Create Pull Request

Create a new pull request from one branch to another.

```graphql
mutation {
  createPullRequest(
    owner: "admin123"
    repo: "my-repo"
    title: "Add user authentication feature"
    body: "## Changes\n- Added JWT auth\n- Added login endpoint\n\n## Testing\n- Manual testing completed"
    head: "feature/user-auth"    # Source branch
    base: "main"                 # Target branch
  ) {
    id
    number
    htmlUrl
    state
    title
  }
}
```

**Branch Format:**
- Simple name: `feature/my-feature`
- With owner: `username:feature/my-feature`

### Update Pull Request

Modify an existing pull request.

```graphql
mutation {
  updatePullRequest(
    owner: "admin123"
    repo: "my-repo"
    number: 42
    title: "Updated: Add user authentication"
    body: "Updated description with more details"
    state: "open"               # open or closed
  ) {
    id
    number
    title
    body
    state
    updatedAt
  }
}
```

### Merge Pull Request

Merge a pull request using various strategies.

```graphql
mutation {
  mergePullRequest(
    owner: "admin123"
    repo: "my-repo"
    number: 42
    method: "squash"             # merge, rebase, rebase-merge, squash
    deleteBranchAfterMerge: true
  )
}
```

**Merge Methods:**
- `merge` - Create a merge commit (default)
- `rebase` - Rebase and fast-forward
- `rebase-merge` - Rebase then create merge commit
- `squash` - Squash commits into one

### Check if PR is Merged

Check the merge status of a pull request.

```graphql
query {
  isPRMerged(
    owner: "admin123"
    repo: "my-repo"
    number: 42
  )
}
```

---

## Comments & Reviews

### List PR Comments

Get all comments on a pull request.

```graphql
query {
  listPRComments(
    owner: "admin123"
    repo: "my-repo"
    number: 42
  ) {
    id
    user {
      login
      fullName
      avatarUrl
    }
    body
    createdAt
    updatedAt
    htmlUrl
  }
}
```

### Create PR Comment

Add a comment to a pull request.

```graphql
mutation {
  createPRComment(
    owner: "admin123"
    repo: "my-repo"
    number: 42
    body: "Great work! Just one small suggestion on line 45."
  ) {
    id
    body
    user {
      login
    }
    createdAt
  }
}
```

### List PR Reviews

Get all reviews on a pull request.

```graphql
query {
  listPRReviews(
    owner: "admin123"
    repo: "my-repo"
    number: 42
  ) {
    id
    user {
      login
      fullName
    }
    body
    state          # APPROVED, REQUEST_CHANGES, COMMENT, PENDING
    commitId
    submittedAt
    htmlUrl
  }
}
```

### Create PR Review

Submit a review on a pull request.

```graphql
mutation {
  createPRReview(
    owner: "admin123"
    repo: "my-repo"
    number: 42
    event: "APPROVE"              # APPROVE, REQUEST_CHANGES, COMMENT
    body: "Looks good to me! ✅"
  ) {
    id
    state
    body
    submittedAt
  }
}
```

**Review Types:**
- `APPROVE` - Approve the changes
- `REQUEST_CHANGES` - Request changes before merging
- `COMMENT` - Comment without explicit approval

---

## File Changes

### List Changed Files

Get all files modified in a pull request.

```graphql
query {
  listPRFiles(
    owner: "admin123"
    repo: "my-repo"
    number: 42
  ) {
    filename
    status        # added, modified, deleted, renamed
    additions
    deletions
    changes
    patchUrl
    rawUrl
    contentsUrl
  }
}
```

**File Status:**
- `added` - New file
- `modified` - Changed file
- `deleted` - Removed file
- `renamed` - File was moved/renamed

### Get PR Diff

Retrieve the full diff of a pull request.

```graphql
query {
  getPRDiff(
    owner: "admin123"
    repo: "my-repo"
    number: 42
  )
}
```

Returns a string containing the unified diff format.

---

## Complete Workflow Examples

### Example 1: Feature Branch Workflow

```graphql
# 1. Create feature branch (use branch API)
mutation {
  createBranch(
    owner: "admin123"
    repo: "backend-api"
    branchName: "feature/add-pagination"
    oldBranchName: "develop"
  ) {
    name
  }
}

# 2. After work is done, create pull request
mutation {
  createPullRequest(
    owner: "admin123"
    repo: "backend-api"
    title: "Add pagination to API endpoints"
    body: "## Changes\n- Added pagination support\n- Updated tests\n\n## Breaking Changes\nNone"
    head: "feature/add-pagination"
    base: "develop"
  ) {
    number
    htmlUrl
  }
}

# 3. Review the PR
query {
  getPullRequest(owner: "admin123", repo: "backend-api", number: 5) {
    title
    mergeable
    additions
    deletions
    changedFiles
  }
}

# 4. Check changed files
query {
  listPRFiles(owner: "admin123", repo: "backend-api", number: 5) {
    filename
    status
    additions
    deletions
  }
}

# 5. Add review
mutation {
  createPRReview(
    owner: "admin123"
    repo: "backend-api"
    number: 5
    event: "APPROVE"
    body: "LGTM! Ready to merge."
  ) {
    state
  }
}

# 6. Merge PR
mutation {
  mergePullRequest(
    owner: "admin123"
    repo: "backend-api"
    number: 5
    method: "squash"
    deleteBranchAfterMerge: true
  )
}
```

### Example 2: Code Review Process

```graphql
# 1. List open PRs
query {
  listPullRequests(
    owner: "admin123"
    repo: "frontend"
    state: "open"
  ) {
    number
    title
    user {
      login
    }
    createdAt
  }
}

# 2. Get PR details
query {
  getPullRequest(owner: "admin123", repo: "frontend", number: 12) {
    title
    body
    head { ref }
    base { ref }
    additions
    deletions
  }
}

# 3. View diff
query {
  getPRDiff(owner: "admin123", repo: "frontend", number: 12)
}

# 4. Add comment
mutation {
  createPRComment(
    owner: "admin123"
    repo: "frontend"
    number: 12
    body: "Can you add tests for the new component?"
  ) {
    id
  }
}

# 5. Request changes
mutation {
  createPRReview(
    owner: "admin123"
    repo: "frontend"
    number: 12
    event: "REQUEST_CHANGES"
    body: "Please address the test coverage before merging."
  ) {
    state
  }
}
```

### Example 3: Hotfix Workflow

```graphql
# 1. Create hotfix branch
mutation {
  createBranch(
    owner: "admin123"
    repo: "production-app"
    branchName: "hotfix/security-patch"
    oldBranchName: "main"
  ) {
    name
  }
}

# 2. Create urgent PR
mutation {
  createPullRequest(
    owner: "admin123"
    repo: "production-app"
    title: "URGENT: Security patch for CVE-2024-1234"
    body: "## Security Fix\n- Patches XSS vulnerability\n- Tested in staging\n\n**Requires immediate merge**"
    head: "hotfix/security-patch"
    base: "main"
  ) {
    number
    htmlUrl
  }
}

# 3. Quick review and approve
mutation {
  createPRReview(
    owner: "admin123"
    repo: "production-app"
    number: 99
    event: "APPROVE"
    body: "Security fix verified. Approving for immediate deployment."
  ) {
    state
  }
}

# 4. Fast-forward merge
mutation {
  mergePullRequest(
    owner: "admin123"
    repo: "production-app"
    number: 99
    method: "rebase"
    deleteBranchAfterMerge: true
  )
}
```

---

## Access Control

All PR operations respect LDAP-based access control:

✅ **Can access PR if:**
- Repository is in user's LDAP `githubRepository` attribute
- OR repository is in user's department's `githubRepository` attribute

❌ **Access denied if:**
- User not authenticated (no JWT)
- Repository not in user's allowed list

---

## Best Practices

### PR Titles
- Be descriptive: "Add user authentication" ✅
- Not: "Fix stuff" ❌
- Use prefixes: `feat:`, `fix:`, `docs:`, `refactor:`

### PR Descriptions
```markdown
## Changes
- Bullet point list of changes
- Keep it concise

## Why
- Explain the problem being solved
- Link to relevant issues

## Testing
- How was this tested?
- Any edge cases covered?

## Breaking Changes
- List any breaking changes
- Or "None"
```

### Review Process
1. **Self-review first** - Review your own changes
2. **Request specific reviewers** - Tag relevant people
3. **Address feedback promptly** - Don't let PRs go stale
4. **Keep PRs small** - Easier to review, faster to merge
5. **One feature per PR** - Don't mix unrelated changes

### Merge Strategies

**Use `merge` when:**
- You want to preserve full history
- Working on long-lived feature branches

**Use `squash` when:**
- Many small commits (WIP commits, fixes)
- Want clean history on main branch
- Most common for feature branches

**Use `rebase` when:**
- Want linear history
- Working with short-lived branches
- CI/CD requires it

**Use `rebase-merge` when:**
- Want linear history but preserve PR grouping
- Good middle ground

---

## Error Handling

### Common Errors

**"Access denied"**
```
User doesn't have access to repository
→ Check LDAP githubRepository attributes
```

**"Pull request not found"**
```
PR number doesn't exist
→ Verify PR number and repository
```

**"Branch not found"**
```
Source or target branch doesn't exist
→ Check branch names
```

**"Cannot merge"**
```
Conflicts or checks failing
→ Resolve conflicts first
→ Ensure CI/CD passes
```

**"PR already merged"**
```
PR was already merged
→ Check PR state before merging
```

---

## Webhooks & Automation

While webhooks aren't directly part of this API, you can build automation:

### Auto-merge Bot
```graphql
# Check if PR is ready
query {
  getPullRequest(...) {
    mergeable
    # Check if reviews are approved
    # Check if CI/CD passes
  }
}

# Auto-merge if conditions met
mutation {
  mergePullRequest(...)
}
```

### PR Size Checker
```graphql
query {
  listPRFiles(...) {
    additions
    deletions
  }
}

# Warn if too large (> 500 lines changed)
```

---

## Performance Tips

1. **Use pagination** - Don't fetch all PRs at once
   ```graphql
   listPullRequests(limit: 30, page: 1)
   ```

2. **Filter by state** - Reduce data transfer
   ```graphql
   listPullRequests(state: "open")
   ```

3. **Request only needed fields** - Don't over-fetch
   ```graphql
   getPullRequest {
     number
     title
     state
     # Only what you need
   }
   ```

---

## cURL Examples

### Create PR

```bash
curl -X POST http://gitea-service:8081/graphql \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation { createPullRequest(owner: \"admin123\", repo: \"my-repo\", title: \"New Feature\", head: \"feature/new\", base: \"main\") { number htmlUrl } }"
  }'
```

### List PRs

```bash
curl -X POST http://gitea-service:8081/graphql \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "query { listPullRequests(owner: \"admin123\", repo: \"my-repo\", state: \"open\") { number title state } }"
  }'
```

### Merge PR

```bash
curl -X POST http://gitea-service:8081/graphql \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation { mergePullRequest(owner: \"admin123\", repo: \"my-repo\", number: 42, method: \"squash\") }"
  }'
```

---

## Summary

The Pull Request API provides complete Git workflow functionality:

✅ Create and manage PRs
✅ Code review with comments and reviews
✅ View diffs and changed files
✅ Multiple merge strategies
✅ Full LDAP-based access control
✅ Webhooks-ready for automation

Perfect for building code review platforms, CI/CD integration, and automated workflows!
