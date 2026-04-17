# LDAP User Federation Setup in Keycloak

This guide explains how to configure OpenLDAP as a user federation provider in Keycloak for the DevPlatform project.

## Overview

LDAP User Federation allows Keycloak to:
- **Authenticate users** against OpenLDAP instead of its own database
- **Sync user data** from LDAP to Keycloak
- **Manage users** centrally in OpenLDAP while using Keycloak for OAuth2/OIDC
- **Federate identities** across multiple services with a single source of truth

## Architecture

```
┌─────────────────────────────────────────────┐
│           Keycloak (OAuth2 Provider)        │
│                                             │
│  ┌───────────────────────────────────────┐ │
│  │    User Federation: LDAP              │ │
│  │    - Sync Mode: LDAP Only             │ │
│  │    - Auth: Bind with User Credentials │ │
│  └───────────────┬───────────────────────┘ │
└──────────────────┼─────────────────────────┘
                   │
                   ▼
         ┌─────────────────┐
         │    OpenLDAP     │
         │                 │
         │ ou=users        │
         │ ou=departments  │
         │ ou=groups       │
         └─────────────────┘
```

## Prerequisites

- Keycloak deployed and accessible
- OpenLDAP running with sample users
- Admin access to Keycloak admin console

## Step-by-Step Setup

### 1. Access Keycloak Admin Console

```
URL: http://192.168.127.2:30080/admin/master/console/
Username: admin
Password: admin_password_change_me
```

### 2. Create or Select Realm

1. Click on the realm dropdown (top-left)
2. If **devplatform** realm doesn't exist:
   - Click **Create Realm**
   - Name: `devplatform`
   - Enabled: ON
   - Click **Create**
3. Switch to **devplatform** realm

### 3. Add LDAP User Federation

1. Navigate to **User Federation** in the left menu
2. Click **Add LDAP providers** dropdown
3. Select **ldap**

### 4. Configure LDAP Connection Settings

#### General Settings

| Setting | Value | Description |
|---------|-------|-------------|
| **Console display name** | `OpenLDAP` | Name shown in admin console |
| **Vendor** | `Other` | LDAP server type |
| **Connection URL** | `ldap://openldap.dev-platform.svc.cluster.local:389` | LDAP server endpoint |
| **Enable StartTLS** | `OFF` | TLS encryption (use ON in production) |
| **Use Truststore SPI** | `ldapsOnly` | Trust store configuration |

#### Authentication Settings

| Setting | Value | Description |
|---------|-------|-------------|
| **Bind Type** | `simple` | Authentication method |
| **Bind DN** | `cn=admin,dc=devplatform,dc=local` | Admin user for LDAP queries |
| **Bind Credential** | `admin123` | Admin password |

#### LDAP Searching and Updating

| Setting | Value | Description |
|---------|-------|-------------|
| **Edit Mode** | `READ_ONLY` or `WRITABLE` | Can Keycloak modify LDAP? |
| **Users DN** | `ou=users,dc=devplatform,dc=local` | Where to find users |
| **User Object Classes** | `inetOrgPerson, posixAccount` | User entry types |
| **RDN LDAP attribute** | `uid` | Username attribute |
| **UUID LDAP attribute** | `entryUUID` | Unique identifier |
| **Username LDAP attribute** | `uid` | Login username field |

#### User Attribute Mappings

| Setting | Value | Description |
|---------|-------|-------------|
| **User LDAP Filter** | (leave empty) | Filter users (optional) |
| **Search Scope** | `One Level` or `Subtree` | Search depth |
| **Read Timeout** | `10000` | Timeout in milliseconds |
| **Pagination** | `ON` | Enable for large datasets |

#### Synchronization Settings

| Setting | Value | Description |
|---------|-------|-------------|
| **Import Users** | `ON` | Sync users from LDAP |
| **Sync Registrations** | `OFF` | Don't write new Keycloak users to LDAP |
| **Batch Size** | `1000` | Users per sync batch |
| **Periodic Full Sync** | `ON` | Regular full sync |
| **Full Sync Period** | `604800` | Sync every 7 days (in seconds) |
| **Periodic Changed Users Sync** | `ON` | Sync modified users |
| **Changed Users Sync Period** | `86400` | Daily sync (in seconds) |

### 5. Test LDAP Connection

1. Click **Test connection** button
   - Expected: ✅ "Success! LDAP connection successful"

2. Click **Test authentication** button
   - Expected: ✅ "Success! LDAP authentication successful"

### 6. Save Configuration

Click **Save** at the bottom of the page.

### 7. Sync Users from LDAP

After saving, two new buttons appear:

1. **Synchronize all users**
   - Imports ALL users from LDAP
   - Click this button now
   - Expected result: "Success! Sync of users finished successfully. X imported users, Y updated users"

2. **Synchronize changed users**
   - Only syncs users modified since last sync
   - Use this for incremental updates

### 8. Configure User Attribute Mappers

Navigate to **Mappers** tab to define how LDAP attributes map to Keycloak:

#### Email Mapper
- **Name**: `email`
- **Mapper Type**: `user-attribute-ldap-mapper`
- **User Model Attribute**: `email`
- **LDAP Attribute**: `mail`
- **Read Only**: `ON` (if LDAP is source of truth)
- **Always Read Value From LDAP**: `ON`
- **Is Mandatory In LDAP**: `OFF`

#### First Name Mapper
- **Name**: `first name`
- **Mapper Type**: `user-attribute-ldap-mapper`
- **User Model Attribute**: `firstName`
- **LDAP Attribute**: `givenName`

#### Last Name Mapper
- **Name**: `last name`
- **Mapper Type**: `user-attribute-ldap-mapper`
- **User Model Attribute**: `lastName`
- **LDAP Attribute**: `sn`

#### Department Mapper (Custom Attribute)
- **Name**: `department`
- **Mapper Type**: `user-attribute-ldap-mapper`
- **User Model Attribute**: `department`
- **LDAP Attribute**: `departmentNumber`

### 9. Verify User Import

1. Navigate to **Users** in the left menu
2. Click **View all users** button
3. You should see LDAP users with federation link icon
4. **Note**: LDAP users are lazy-loaded:
   - Search by username to see them
   - Or login once to cache them

### 10. Test User Authentication

Try logging in with an LDAP user:

```bash
curl -X POST http://localhost:30080/realms/devplatform/protocol/openid-connect/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password" \
  -d "client_id=gitea-service" \
  -d "client_secret=G6KVMjriWXCdPqLDjWrDWaIfTtYQOwtO" \
  -d "username=john.doe" \
  -d "password=password123"
```

Expected: JSON response with `access_token`

## LDAP Schema Reference

### User Entry Example

```ldif
dn: uid=john.doe,ou=users,dc=devplatform,dc=local
objectClass: inetOrgPerson
objectClass: posixAccount
objectClass: shadowAccount
objectClass: extensibleObject
uid: john.doe
cn: John Doe Doe
sn: Doe
givenName: John Doe
mail: john.doe@devplatform.local
departmentNumber: engineering
uidNumber: 10001
gidNumber: 10001
homeDirectory: /home/john.doe
userPassword: {SSHA}...
```

## Troubleshooting

### Issue: "Connection timeout"
**Solution**:
- Verify OpenLDAP is running: `kubectl get pods -n dev-platform`
- Check service endpoint: `kubectl get svc -n dev-platform openldap`
- Test connectivity: `kubectl exec -it <keycloak-pod> -- ldapsearch -H ldap://openldap.dev-platform.svc.cluster.local:389 -x`

### Issue: "Invalid credentials"
**Solution**:
- Verify bind DN: `cn=admin,dc=devplatform,dc=local`
- Check password in secret: `kubectl get secret openldap-admin-secret -n dev-platform -o yaml`
- Password should be: `admin123` (base64 encoded in secret)

### Issue: "No users imported"
**Solution**:
- Verify Users DN: `ou=users,dc=devplatform,dc=local`
- Check users exist:
  ```bash
  kubectl exec -it <openldap-pod> -n dev-platform -- \
    ldapsearch -x -b "ou=users,dc=devplatform,dc=local" -D "cn=admin,dc=devplatform,dc=local" -w admin123
  ```
- Verify User Object Classes match: `inetOrgPerson, posixAccount`

### Issue: "Users not showing in list"
**Solution**:
- LDAP users are lazy-loaded - search by username
- Click "View all users" might not show federated users
- Search for specific user: `john.doe`

## Security Best Practices

### Production Recommendations

1. **Enable TLS/StartTLS**
   - Use `ldaps://` (port 636) instead of `ldap://`
   - Or enable StartTLS on port 389

2. **Use Read-Only Service Account**
   - Don't use admin credentials for Keycloak bind
   - Create dedicated service account:
     ```ldif
     dn: cn=keycloak-readonly,ou=services,dc=devplatform,dc=local
     objectClass: simpleSecurityObject
     objectClass: organizationalRole
     cn: keycloak-readonly
     userPassword: {SSHA}...
     description: Read-only account for Keycloak user federation
     ```

3. **Set Edit Mode to READ_ONLY**
   - Prevents Keycloak from modifying LDAP
   - Use LDAP Manager service for user management

4. **Configure User Filters**
   - Only sync active users: `(!(userAccountControl:1.2.840.113556.1.4.803:=2))`
   - Filter by department: `(departmentNumber=engineering)`

5. **Enable Periodic Sync**
   - Keep Keycloak cache fresh
   - Recommended: Full sync weekly, changed users daily

## Features Enabled by LDAP Federation

✅ **Single Source of Truth**: All user data in OpenLDAP
✅ **Centralized Management**: Use LDAP Manager service to manage users
✅ **OAuth2/OIDC**: Keycloak provides modern auth on top of LDAP
✅ **Microservice Auth**: gitea-service and ldap-manager-service use same tokens
✅ **No Password Duplication**: Users authenticate against LDAP
✅ **Automatic Sync**: New LDAP users available in Keycloak within 24h
✅ **Lazy Loading**: LDAP users loaded on-demand (first login or search)

## Integration with Services

### gitea-service
- Receives JWT from Keycloak with `preferred_username` claim
- Maps to LDAP `uid` attribute
- Uses for Gitea reverse proxy authentication

### ldap-manager-service
- Receives same JWT token
- Extracts user context for LDAP operations
- Manages users in OpenLDAP directly

### Frontend Application
- Uses Keycloak login page
- Gets OAuth2 access token
- Sends token to microservices

## Next Steps

After LDAP federation is configured:

1. **Create OAuth2 clients** for each microservice (see `keycloak-client-template.json`)
2. **Configure Istio** RequestAuthentication for JWT validation
3. **Test end-to-end flow** from login to API call
4. **Set up frontend** with Keycloak JavaScript adapter
5. **Monitor sync jobs** in Keycloak admin console

## References

- [Keycloak LDAP User Federation Documentation](https://www.keycloak.org/docs/latest/server_admin/#_ldap)
- [OpenLDAP Schema Reference](https://www.openldap.org/doc/admin24/schema.html)
- Project documentation: `KEYCLOAK.md`, `OAUTH2_IMPLEMENTATION.md`
