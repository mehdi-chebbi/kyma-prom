# DevPlatform Postman Collection

Complete Postman collection for testing DevPlatform microservices with **automatic OAuth2 token management**. No manual token copying required!

## Features

✅ **Auto Token Management** - Login once, tokens saved automatically
✅ **Pre-configured Endpoints** - All GraphQL queries ready to use
✅ **Environment Variables** - Easy switching between environments
✅ **Team Ready** - Share with your team, no setup needed
✅ **LDAP & Gitea** - Full coverage of both microservices

## Quick Start

### 1. Import Collection

1. Open Postman
2. Click **Import** (top-left)
3. Drag and drop these files:
   - `DevPlatform.postman_collection.json`
   - `DevPlatform.postman_environment.json`
4. Click **Import**

### 2. Select Environment

1. Click environment dropdown (top-right)
2. Select **DevPlatform - Local**

### 3. Login (Get Token)

1. Open **Auth** folder
2. Run **Login (Get Token)** request
3. ✅ Token automatically saved!
4. All other requests now work automatically

## Collection Structure

```
DevPlatform - OAuth2 Microservices
├── Auth
│   ├── Login (Get Token)         ← Start here!
│   ├── Refresh Token
│   ├── Get OpenID Configuration
│   └── Logout
├── LDAP Manager Service
│   ├── Users
│   │   ├── List All Users
│   │   ├── Get User by UID
│   │   ├── Create User
│   │   ├── Update User
│   │   └── Delete User
│   ├── Departments
│   │   ├── List All Departments
│   │   ├── Get Department
│   │   └── Create Department
│   └── Health
│       ├── Health Check
│       └── Readiness Check
└── Gitea Service
    ├── Repositories
    │   ├── List Repositories
    │   ├── Get Repository
    │   └── Create Repository
    ├── Issues
    │   ├── List Issues
    │   └── Create Issue
    └── Health
        ├── Health Check
        └── Readiness Check
```

## Environment Variables

The environment file includes all necessary configuration:

| Variable | Default Value | Description |
|----------|--------------|-------------|
| `keycloak_url` | `http://localhost:30080` | Keycloak server URL |
| `realm` | `devplatform` | Keycloak realm name |
| `client_id` | `gitea-service` | OAuth2 client ID |
| `client_secret` | `G6KVMjriWXCdPqLDjWrDWaIfTtYQOwtO` | OAuth2 client secret |
| `username` | `john.doe` | Default test user |
| `password` | `password123` | Default test password |
| `gitea_service_url` | `http://localhost:30011` | Gitea service endpoint |
| `ldap_manager_url` | `http://localhost:30012` | LDAP Manager endpoint |
| `access_token` | (auto-filled) | OAuth2 access token |
| `refresh_token` | (auto-filled) | OAuth2 refresh token |
| `token_expires_in` | (auto-filled) | Token expiration time |
| `token_expires_at` | (auto-filled) | Token expiration timestamp |

## How It Works

### Automatic Token Management

1. **First Login**: Run "Login (Get Token)" request
   - Sends credentials to Keycloak
   - Response contains `access_token` and `refresh_token`
   - Test script automatically saves tokens to environment

2. **Subsequent Requests**: All other requests use saved token
   - Authorization header: `Bearer {{access_token}}`
   - No manual copying needed!

3. **Token Refresh**: When token expires (5 minutes)
   - Run "Refresh Token" request
   - New tokens automatically saved

### Test Scripts

Each request includes smart test scripts:

**Login Request:**
```javascript
// Automatically saves tokens to environment
var jsonData = pm.response.json();
pm.environment.set("access_token", jsonData.access_token);
pm.environment.set("refresh_token", jsonData.refresh_token);
pm.environment.set("token_expires_at", ...);
```

**Pre-request Script:**
```javascript
// Warns when token is expired
// Suggests running refresh token request
```

## Usage Examples

### Example 1: List All LDAP Users

1. Ensure you're logged in (run "Login" if not)
2. Navigate to **LDAP Manager Service → Users → List All Users**
3. Click **Send**
4. ✅ See all users from OpenLDAP

### Example 2: Create a New Repository

1. Navigate to **Gitea Service → Repositories → Create Repository**
2. Edit variables in the request body:
   ```json
   {
     "input": {
       "name": "my-awesome-project",
       "description": "My new project",
       "private": false,
       "autoInit": true
     }
   }
   ```
3. Click **Send**
4. ✅ Repository created in Gitea

### Example 3: Create LDAP User

1. Navigate to **LDAP Manager Service → Users → Create User**
2. Edit the GraphQL variables:
   ```json
   {
     "input": {
       "uid": "jane.smith",
       "cn": "Jane Smith",
       "sn": "Smith",
       "givenName": "Jane",
       "mail": "jane.smith@devplatform.local",
       "password": "securepassword",
       "department": "engineering",
       "repositories": ["https://github.com/org/project"]
     }
   }
   ```
3. Click **Send**
4. ✅ User created in OpenLDAP

## Customizing for Your Team

### Create Additional Environments

You can create multiple environments for different setups:

**DevPlatform - Production:**
```json
{
  "keycloak_url": "https://keycloak.production.com",
  "gitea_service_url": "https://gitea-api.production.com",
  "ldap_manager_url": "https://ldap-api.production.com",
  "username": "your.username",
  "password": "your.password"
}
```

**DevPlatform - Staging:**
```json
{
  "keycloak_url": "http://staging-server:30080",
  "gitea_service_url": "http://staging-server:30011",
  "ldap_manager_url": "http://staging-server:30012"
}
```

### Change Default User

Edit environment variables:
```json
{
  "username": "taztaz",
  "password": "taztaz123"
}
```

### Use Different OAuth2 Client

For LDAP Manager service client:
```json
{
  "client_id": "ldap-manager-service",
  "client_secret": "YOUR_LDAP_MANAGER_SECRET"
}
```

## Troubleshooting

### Issue: "401 Unauthorized"

**Cause**: Token expired or not set
**Solution**:
1. Run **Auth → Login (Get Token)**
2. Check environment shows token value (not empty)
3. Verify token hasn't expired (check `token_expires_at`)

### Issue: "Invalid credentials"

**Cause**: Wrong username/password
**Solution**:
1. Check environment variables `username` and `password`
2. Verify user exists in LDAP:
   ```bash
   kubectl exec -it <ldap-pod> -- ldapsearch -x -b "ou=users,dc=devplatform,dc=local" uid=john.doe
   ```
3. Verify LDAP federation is synced in Keycloak

### Issue: "Connection refused"

**Cause**: Services not accessible
**Solution**:
1. Check Kubernetes pods are running:
   ```bash
   kubectl get pods -n auth-system
   kubectl get pods -n dev-platform
   ```
2. Verify LoadBalancer IPs:
   ```bash
   kubectl get svc -n auth-system keycloak-external
   kubectl get svc -n dev-platform gitea-service
   kubectl get svc -n dev-platform ldap-manager
   ```
3. Update environment URLs if needed

### Issue: Token not auto-saving

**Cause**: Test script not running
**Solution**:
1. Check Postman console (View → Show Postman Console)
2. Look for "✅ Token saved successfully" message
3. If missing, check test script in request

## Advanced Usage

### Using Variables in Requests

All requests support environment variables with `{{variable}}` syntax:

```graphql
query GetUser {
  user(uid: "{{username}}") {
    uid
    mail
  }
}
```

### Collection Variables

You can add collection-level variables for shared data:

1. Click collection name
2. Go to **Variables** tab
3. Add variables (e.g., `default_department`, `test_repo_name`)

### Pre-request Scripts

Add custom logic before each request:

```javascript
// Set dynamic timestamp
pm.environment.set("timestamp", new Date().toISOString());

// Generate random user
pm.environment.set("random_uid", "user_" + Math.random().toString(36).substr(2, 9));
```

### Tests and Assertions

Validate responses automatically:

```javascript
// Check status code
pm.test("Status is 200", function () {
    pm.response.to.have.status(200);
});

// Check GraphQL response
pm.test("Has data", function () {
    var jsonData = pm.response.json();
    pm.expect(jsonData.data).to.not.be.null;
});

// Check user was created
pm.test("User created successfully", function () {
    var jsonData = pm.response.json();
    pm.expect(jsonData.data.createUser.uid).to.eql(pm.environment.get("test_uid"));
});
```

## Team Collaboration

### Sharing Collections

**Option 1: Export/Import**
1. Right-click collection → Export
2. Share JSON file with team
3. Team members import the file

**Option 2: Postman Workspace**
1. Create a team workspace in Postman
2. Move collection to workspace
3. Team members sync automatically

### Version Control

Commit Postman files to Git:
```bash
git add postman/
git commit -m "Add Postman collection for API testing"
git push
```

Team members can pull latest version:
```bash
git pull
# Re-import updated files in Postman
```

### Environment Per Developer

Each developer can create their own environment:
- **DevPlatform - Alice**
- **DevPlatform - Bob**
- **DevPlatform - Charlie**

With personalized credentials and endpoints.

## Security Notes

⚠️ **Important Security Considerations:**

1. **Never commit secrets to Git**
   - Use `.gitignore` for environment files with real credentials
   - Share template environment without actual secrets

2. **Use environment variables for secrets**
   - Mark sensitive fields as "secret" type in Postman
   - Secrets are masked in Postman UI

3. **Production credentials**
   - Never use production credentials in shared collections
   - Create separate environments for production with restricted access

4. **Client secrets**
   - Rotate OAuth2 client secrets regularly
   - Use different secrets per environment

## FAQ

**Q: Do I need to login every time I open Postman?**
A: No! Tokens persist in environment as long as you don't clear them. When token expires (5 min), just run "Refresh Token".

**Q: Can multiple team members use the same credentials?**
A: Yes, but each should create their own environment with their username/password for better tracking.

**Q: How do I test with different users?**
A: Change `username` and `password` in environment, then run "Login" again.

**Q: Can I use this with Istio?**
A: Yes! The microservices support both direct JWT (Postman) and Istio-injected headers. No changes needed.

**Q: What happens when I logout?**
A: The "Logout" request invalidates tokens on Keycloak. You'll need to login again.

**Q: Can I automate testing?**
A: Yes! Use Postman's Collection Runner or Newman (CLI) for automated testing:
```bash
newman run DevPlatform.postman_collection.json -e DevPlatform.postman_environment.json
```

## Next Steps

1. ✅ Import collection and environment
2. ✅ Run login request
3. ✅ Test LDAP and Gitea endpoints
4. ✅ Share with your team
5. ✅ Create custom environments for different setups
6. ✅ Add your own requests and tests

## Support

For issues or questions:
- Check the main project documentation: `KEYCLOAK.md`, `OAUTH2_IMPLEMENTATION.md`
- Review Keycloak admin console: http://localhost:30080/admin
- Check service logs:
  ```bash
  kubectl logs -f -l app=gitea-service -n dev-platform
  kubectl logs -f -l app=ldap-manager -n dev-platform
  ```

Happy testing! 🚀
