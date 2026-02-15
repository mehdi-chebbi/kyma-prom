# Postman Quick Start - 5 Minutes Setup

Get started testing DevPlatform APIs in 5 minutes with automatic token management!

## Step 1: Import Files (1 minute)

1. Open **Postman**
2. Click **Import** button (top-left)
3. Drag these 2 files into Postman:
   - `DevPlatform.postman_collection.json`
   - `DevPlatform.postman_environment.json`
4. Click **Import**

✅ You should now see:
- Collection: "DevPlatform - OAuth2 Microservices" in left sidebar
- Environment: "DevPlatform - Local" in top-right dropdown

## Step 2: Select Environment (30 seconds)

1. Click environment dropdown (top-right corner)
2. Select **DevPlatform - Local**
3. Click the eye icon 👁️ to view variables

✅ You should see variables like:
- keycloak_url: `http://localhost:30080`
- username: `john.doe`
- password: `password123`

## Step 3: Login (1 minute)

1. In left sidebar, expand **Auth** folder
2. Click **Login (Get Token)**
3. Click **Send** button
4. Wait for response (should be ~2 seconds)

✅ Expected response:
```json
{
  "access_token": "eyJhbGc...",
  "refresh_token": "eyJhbGc...",
  "expires_in": 300,
  "token_type": "Bearer"
}
```

**Success indicators:**
- ✅ Status: 200 OK
- ✅ Console shows: "✅ Token saved successfully"
- ✅ Environment now has `access_token` value (check eye icon 👁️)

## Step 4: Test an Endpoint (1 minute)

Let's test the LDAP Manager service:

1. Expand **LDAP Manager Service → Users**
2. Click **List All Users**
3. Click **Send**

✅ Expected response:
```json
{
  "data": {
    "usersAll": [
      {
        "uid": "john.doe",
        "cn": "John Doe Doe",
        "mail": "john.doe@devplatform.local",
        "department": "engineering",
        ...
      },
      ...
    ]
  }
}
```

## Step 5: Test Gitea Service (1 minute)

1. Expand **Gitea Service → Repositories**
2. Click **List Repositories**
3. Click **Send**

✅ Expected response:
```json
{
  "data": {
    "listRepositories": {
      "items": [
        {
          "id": 1,
          "name": "test-repo",
          "owner": {
            "login": "john.doe"
          },
          ...
        }
      ],
      "total": 1
    }
  }
}
```

## 🎉 You're Done!

All endpoints are now ready to use with automatic authentication!

## Common Tasks

### Create a New User

1. **LDAP Manager Service → Users → Create User**
2. Edit variables in request body:
   ```json
   {
     "input": {
       "uid": "test.user",
       "cn": "Test User",
       "mail": "test@devplatform.local",
       "password": "testpass123",
       "department": "engineering"
     }
   }
   ```
3. Send

### Create a Repository

1. **Gitea Service → Repositories → Create Repository**
2. Edit variables:
   ```json
   {
     "input": {
       "name": "my-project",
       "description": "My awesome project",
       "private": false
     }
   }
   ```
3. Send

### Check Service Health

1. **LDAP Manager Service → Health → Health Check** (or Gitea Service)
2. Send
3. Should return: `{"status": "ok"}`

## When Token Expires (Every 5 Minutes)

You'll see **401 Unauthorized** errors. Simply:

1. Go to **Auth → Refresh Token**
2. Click **Send**
3. ✅ New token saved automatically
4. Try your request again

## Troubleshooting

### Problem: "401 Unauthorized"

**Solution:** Run **Auth → Login (Get Token)** again

### Problem: "Connection refused"

**Solution:** Make sure services are running:
```bash
kubectl get pods -n dev-platform
kubectl get pods -n auth-system
```

### Problem: "Invalid credentials"

**Solution:** Check environment variables:
- username: `john.doe`
- password: `password123`

Or try another user:
- username: `taztaz`
- password: `taztaz123`

## Next Steps

- Read full documentation: `README.md`
- Customize environment variables for your setup
- Share collection with team members
- Create your own environments (Dev, Staging, Prod)

## Tips

💡 **Collection Runner**: Run all requests automatically
1. Click collection name
2. Click **Run**
3. Select requests to run
4. Click **Run DevPlatform...**

💡 **Postman Console**: See detailed logs
- View → Show Postman Console
- See token management logs

💡 **Variables**: Use `{{variable}}` in any request
- Example: `{{username}}`, `{{keycloak_url}}`

💡 **Environments**: Switch between Local/Staging/Prod
- Top-right dropdown
- Each can have different credentials/URLs

## Support

Questions? Check:
- `postman/README.md` - Full documentation
- `KEYCLOAK.md` - Authentication setup
- `OAUTH2_IMPLEMENTATION.md` - OAuth2 flow details

Happy testing! 🚀
