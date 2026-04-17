# Postman Collection Usage Guide

## Import Collection

1. Open Postman
2. Click **Import** button
3. Select `postman_collection.json`
4. The collection "Gitea Service - GraphQL API Tests" will be imported

## Configure Variables

Before running requests, configure the collection variables:

1. Click on the collection name
2. Go to **Variables** tab
3. Set the following values:

| Variable | Default Value | Description |
|----------|---------------|-------------|
| `baseUrl` | `http://localhost:8080` | Your GraphQL server URL |
| `authToken` | (empty) | JWT token from LDAP Manager login |
| `testOwner` | `testuser` | Repository owner username for tests |
| `testRepo` | `test-repository` | Repository name for tests |
| `orgName` | `test-org` | Organization name for team tests |

## Getting Auth Token

Most operations require authentication. To get an auth token:

1. Login via LDAP Manager or use an existing JWT token
2. Copy the token value
3. Paste it into the `authToken` collection variable
4. Save the collection

The token will be automatically included in all requests that need authentication.

## Collection Structure

The collection is organized into folders by functionality:

### 1. Health & Stats (2 requests)
- Health Check - Test service health
- Repository Stats - Get repository statistics

### 2. Repository Queries (4 requests)
- List Repositories - Paginated repository list
- Search Repositories - Search repos by query
- Get Repository - Get specific repository details
- My Repositories - Get authenticated user's repos

### 3. Repository Mutations (5 requests)
- Create Repository - Create new repo
- Update Repository - Update repo details
- Fork Repository - Fork existing repo
- Migrate Repository - Migrate from external source
- Delete Repository - Delete a repo

### 4. Branch Operations (4 requests)
- List Branches - Get all branches
- Get Branch - Get specific branch
- Create Branch - Create new branch
- Delete Branch - Delete a branch

### 5. Commit & Tag Operations (5 requests)
- List Commits - Get commit history
- Get Commit - Get specific commit
- List Tags - Get all tags
- Create Tag - Create new tag
- Delete Tag - Delete a tag

### 6. Pull Request Operations (12 requests)
- List Pull Requests - Get all PRs
- Get Pull Request - Get specific PR
- Create Pull Request - Create new PR
- Update Pull Request - Update PR details
- Merge Pull Request - Merge a PR
- Is PR Merged - Check if PR is merged
- List PR Comments - Get PR comments
- Create PR Comment - Add comment to PR
- List PR Reviews - Get PR reviews
- Create PR Review - Add review to PR
- List PR Files - Get changed files
- Get PR Diff - Get PR diff content

### 7. Issue Operations (10 requests)
- List Issues - Get all issues
- Get Issue - Get specific issue
- Create Issue - Create new issue
- Update Issue - Update issue details
- List Issue Comments - Get issue comments
- Create Issue Comment - Add comment to issue
- List Labels - Get repository labels
- Create Label - Create new label
- List Milestones - Get milestones
- Create Milestone - Create new milestone

### 8. User Sync Operations (4 requests)
- Get Gitea User - Get user by username
- Search Gitea Users - Search users
- Sync LDAP User - Sync single LDAP user to Gitea
- Sync All LDAP Users - Sync all LDAP users to Gitea

### 9. Team Operations (8 requests)
- List Teams - Get organization teams
- Get Team - Get team details with members
- Create Team - Create new team
- Add Team Member - Add user to team
- Remove Team Member - Remove user from team
- Add Team Repository - Add repo to team
- Remove Team Repository - Remove repo from team
- Sync LDAP Group to Team - Sync LDAP group to Gitea team

## Running Tests

### Test Individual Requests

1. Select a request from the collection
2. Review/modify variables in the request body
3. Click **Send**
4. View response in the response panel

### Test Entire Folder

1. Right-click on a folder (e.g., "Repository Queries")
2. Select **Run folder**
3. Configure run settings
4. Click **Run**

### Test Full Collection

1. Click on collection name
2. Click **Run** button
3. Select requests to run
4. Click **Run Gitea Service - GraphQL API Tests**

## Example Workflow

### 1. Create and Test a Repository

```
1. Run: Health Check (verify service is running)
2. Run: Create Repository (create "my-test-repo")
3. Update variables: testRepo = "my-test-repo"
4. Run: Get Repository (verify creation)
5. Run: Create Branch (create feature branch)
6. Run: List Branches (see all branches)
```

### 2. Create and Merge a Pull Request

```
1. Run: Create Branch (create "feature/new-feature")
2. Run: Create Pull Request (from feature branch to main)
3. Run: Create PR Comment (add review comment)
4. Run: Create PR Review (approve the PR)
5. Run: Merge Pull Request (merge to main)
6. Run: Is PR Merged (verify merge)
```

### 3. Sync LDAP Users and Create Team

```
1. Run: Sync All LDAP Users (sync users from LDAP)
2. Run: Create Team (create "developers" team)
3. Run: Add Team Member (add users to team)
4. Run: Add Team Repository (give team access to repos)
5. Run: Get Team (verify team setup)
```

## Common Variables in Requests

All requests use GraphQL variables that can be modified:

```json
{
  "owner": "{{testOwner}}",
  "repo": "{{testRepo}}",
  "limit": 10,
  "offset": 0
}
```

You can edit these directly in the **Variables** section of each request.

## Response Examples

### Successful Response
```json
{
  "data": {
    "getRepository": {
      "id": 123,
      "name": "test-repo",
      "fullName": "testuser/test-repo",
      "description": "Test repository"
    }
  }
}
```

### Error Response
```json
{
  "errors": [
    {
      "message": "unauthorized",
      "locations": [{"line": 2, "column": 3}],
      "path": ["getRepository"]
    }
  ]
}
```

## Troubleshooting

### "unauthorized" errors
- Ensure `authToken` variable is set correctly
- Token may have expired, get a new one
- Check Authorization header is being sent

### "Repository not found" errors
- Verify `testOwner` and `testRepo` variables are correct
- Ensure repository exists in Gitea
- Check user has access to the repository

### Connection errors
- Verify `baseUrl` is correct
- Ensure gitea-service is running
- Check network connectivity

## Tips

1. **Use Environment Variables**: Create different environments (dev, staging, prod) with different baseUrls
2. **Save Responses**: Save successful responses as examples for reference
3. **Chain Requests**: Use Postman's test scripts to extract data and chain requests
4. **Monitor Performance**: Check response times in the response panel
5. **Export Results**: Export test run results for reporting

## GraphQL vs REST

Note: This is a GraphQL API, not REST:
- All requests go to `/graphql` endpoint
- Use POST method for all requests
- Queries retrieve data, Mutations modify data
- You can request exactly the fields you need

## Further Reading

- [GraphQL Documentation](https://graphql.org/learn/)
- [Postman GraphQL Support](https://learning.postman.com/docs/sending-requests/supported-api-frameworks/graphql/)
- [Gitea API Documentation](https://docs.gitea.io/en-us/api-usage/)
