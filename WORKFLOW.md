# KYMA Flow - Complete Workflow Documentation

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Frontend (React)                         │
│  Login → Dashboard → Repository List → User Management      │
└──────────────────┬──────────────────────────────────────────┘
                   │ HTTP/HTTPS
                   ▼
┌─────────────────────────────────────────────────────────────┐
│              Istio Ingress Gateway                          │
│  - TLS Termination                                          │
│  - Load Balancing                                           │
│  - Routing Rules                                            │
└──────────────────┬──────────────────────────────────────────┘
                   │
         ┌─────────┴─────────┐
         │                   │
         ▼                   ▼
┌───────────────────────────────────────────────────────────┐
│            Istio Service Mesh (mTLS Encrypted)            │
│  ┌─────────────────────┐    ┌─────────────────────┐      │
│  │ LDAP Manager        │    │ Gitea Service       │      │
│  │ [app | istio-proxy] │◄──►│ [app | istio-proxy] │      │
│  │ Port: 30008 (LB)    │    │ Port: 30011 (LB)    │      │
│  │ Internal: 8080      │    │ Internal: 8081      │      │
│  └──────────┬──────────┘    └──────────┬──────────┘      │
│             │ mTLS                      │ mTLS            │
│             ▼                           ▼                 │
│  ┌──────────────────┐       ┌──────────────────┐         │
│  │   OpenLDAP       │       │  Gitea Server    │         │
│  │  (Port 30000)    │       │  (Port 30009)    │         │
│  │  ClusterIP: 389  │       │  ClusterIP: 3000 │         │
│  └──────────────────┘       └──────────────────┘         │
└───────────────────────────────────────────────────────────┘

Legend:
[app | istio-proxy] = Pod with application + Istio sidecar
◄──► = Mutual TLS encrypted communication
```

## Istio Service Mesh Integration

### What Istio Provides

1. **Mutual TLS (mTLS)**
   - All service-to-service communication is encrypted
   - Automatic certificate management and rotation
   - No code changes required

2. **Traffic Management**
   - Intelligent routing (VirtualService)
   - Load balancing (DestinationRule)
   - Circuit breaking and retries
   - Fault injection for testing

3. **Observability**
   - Distributed tracing (Jaeger)
   - Metrics collection (Prometheus)
   - Service topology visualization (Kiali)
   - Access logs

4. **Security**
   - JWT validation (RequestAuthentication)
   - Authorization policies (AuthorizationPolicy)
   - PeerAuthentication (mTLS enforcement)
   - Gateway TLS termination

### Istio Components Deployed

#### For LDAP Manager (`backend/k8s/04-istio-config.yaml`)
- **Gateway**: `ldap-manager-gateway` (ports 80/443)
- **VirtualService**: Routes `/graphql`, `/health`, `/ready`
- **DestinationRule**: Connection pooling, ISTIO_MUTUAL TLS
- **PeerAuthentication**: STRICT mTLS mode
- **AuthorizationPolicy**: Public health checks, authenticated GraphQL
- **ServiceEntry**: OpenLDAP access

#### For Gitea Service (`gitea-service/k8s/06-istio-config.yaml`)
- **Gateway**: `gitea-service-gateway` (ports 80/443)
- **VirtualService**: Routes `/graphql`, `/health`, `/ready`
- **DestinationRule**: Connection pooling, ISTIO_MUTUAL TLS
- **PeerAuthentication**: STRICT mTLS mode
- **AuthorizationPolicy**: Public health checks, authenticated GraphQL
- **ServiceEntry**: External Gitea access

### Traffic Flow with Istio

```
External Request → Istio Ingress Gateway
                 ↓
              VirtualService (routing rules)
                 ↓
              DestinationRule (load balancing)
                 ↓
              Envoy Proxy (mTLS encryption)
                 ↓
              Application Container
```

## Current Deployment Status

✅ **Istio Service Mesh**
- Istio CRDs deployed (if installed)
- Namespace `dev-platform` has `istio-injection=enabled` label
- Ingress Gateway available for external traffic

✅ **OpenLDAP** - Running on port 30000
- Base DN: `dc=devplatform,dc=local`
- Admin user: `cn=admin,dc=devplatform,dc=local`
- Contains test user: `john.doe / password123`
- Service type: ClusterIP (internal only)

✅ **LDAP Manager** - Running on port 30008 (with Istio sidecar)
- Pod containers: `ldap-manager` + `istio-proxy` (2/2)
- LoadBalancer service on port 30008
- Internal service port: 8080
- GraphQL API endpoint: `http://localhost:30008/graphql`
- JWT authentication enabled
- Connected to OpenLDAP via mTLS mesh
- Istio configs: Gateway, VirtualService, DestinationRule, PeerAuth, AuthzPolicy

✅ **Gitea Server** - Running on port 30009
- Web UI: `http://localhost:30009`
- LoadBalancer service on port 30009
- Internal service port: 3000
- Admin user: `admin123 / admin123`
- Needs initial repository setup

⚠️ **Gitea Service** - Running on port 30011 (with Istio sidecar - needs token fix)
- Pod containers: `gitea-service` + `istio-proxy` (2/2)
- LoadBalancer service on port 30011
- Internal service port: 8081
- GraphQL API endpoint: `http://localhost:30011/graphql`
- Currently failing health check (401 - needs valid Gitea token)
- Connected to LDAP Manager via mTLS mesh
- Istio configs: Gateway, VirtualService, DestinationRule, PeerAuth, AuthzPolicy

✅ **Frontend** - Development server
- Running on port 5173
- GraphQL endpoint configured: `http://localhost:30008/graphql`
- Login page set as default route
- Can access services via LoadBalancer or Istio Ingress Gateway

---

## Complete Workflow: Login to Dashboard

### Phase 1: User Authentication Flow

#### Step 1.1: User Opens Frontend
1. User navigates to `http://localhost:5173`
2. Frontend redirects to `/login` (already configured)
3. Login page displays with `uid` and `password` fields

#### Step 1.2: User Enters Credentials
```
Username (UID): john.doe
Password: password123
```

#### Step 1.3: Frontend Sends Login Mutation
```graphql
mutation Login {
  login(uid: "john.doe", password: "password123") {
    token
    user {
      uid
      cn
      mail
      department
      repositories
      uidNumber
      gidNumber
    }
  }
}
```

**Request to:** `http://localhost:30008/graphql`

#### Step 1.4: LDAP Manager Authenticates
1. LDAP Manager receives mutation
2. Connects to OpenLDAP on port 30000
3. Performs LDAP bind with user DN: `uid=john.doe,ou=users,dc=devplatform,dc=local`
4. If successful, generates JWT token
5. Queries user attributes from LDAP
6. Returns `AuthPayload` with token + user data

#### Step 1.5: Frontend Stores Auth Data
```javascript
localStorage.setItem('authToken', result.login.token)
localStorage.setItem('user', JSON.stringify(result.login.user))
```

#### Step 1.6: Frontend Redirects to Dashboard
Navigate to `/dashboard` with authenticated state

---

### Phase 2: Dashboard - Display User Info

#### Step 2.1: Dashboard Loads User Profile
1. Read user data from `localStorage`
2. Display user card with:
   - Full name (cn)
   - Email (mail)
   - Department (department)
   - UID number (uidNumber)

#### Step 2.2: Fetch Current User from API (Optional Refresh)
```graphql
query Me {
  me {
    uid
    cn
    mail
    department
    repositories
  }
}
```

**Headers:**
```
Authorization: Bearer <JWT_TOKEN>
```

**Request to:** `http://localhost:30008/graphql`

---

### Phase 3: Repository List - Display Assigned Repos

#### Step 3.1: Current State (LDAP Only)
User object from LDAP Manager contains `repositories` array:
```json
{
  "uid": "john.doe",
  "repositories": [
    "https://github.com/org/repo1",
    "https://github.com/org/repo2"
  ]
}
```

**Display in Dashboard:**
- Simple list of repository URLs
- These are stored as LDAP attributes
- No live data from Gitea yet

#### Step 3.2: Enhanced Version (Gitea Integration)
**Goal:** Fetch actual repository metadata from Gitea

**Option A: Query Gitea Service Directly**
```graphql
query GetUserRepositories {
  userRepositories(uid: "john.doe") {
    id
    name
    fullName
    description
    htmlUrl
    cloneUrl
    language
    stars
    forks
    updatedAt
  }
}
```

**Request to:** `http://localhost:30011/graphql`

**Backend Flow:**
1. Gitea Service receives query
2. Calls LDAP Manager to get user's `repositories` attribute
3. For each repository URL, calls Gitea API
4. Enriches with live metadata (stars, forks, last commit, etc.)
5. Returns enriched repository list

**Option B: Combined Query (Single Endpoint)**
Modify LDAP Manager to also query Gitea Service:
```graphql
query Me {
  me {
    uid
    cn
    repositories {
      url
      name
      stars
      forks
      lastUpdate
    }
  }
}
```

---

### Phase 4: Repository Assignment Workflow

#### Step 4.1: Admin Assigns Repository to User
**Admin Interface Action:**
1. Navigate to User Management
2. Select user: `john.doe`
3. Click "Assign Repository"
4. Enter repository URLs

**GraphQL Mutation:**
```graphql
mutation AssignRepos {
  assignRepoToUser(
    uid: "john.doe"
    repositories: [
      "https://github.com/devplatform/backend",
      "https://github.com/devplatform/frontend"
    ]
  ) {
    uid
    repositories
  }
}
```

**Request to:** `http://localhost:30008/graphql`

**Backend Flow:**
1. LDAP Manager receives mutation
2. Updates LDAP entry for user
3. Modifies `githubRepository` multi-value attribute
4. Returns updated user

#### Step 4.2: User Sees Updated Repository List
1. User refreshes dashboard
2. Query `me` endpoint
3. New repositories appear in list

---

### Phase 5: Department-Based Repository Access

#### Step 5.1: Assign Repositories to Department
```graphql
mutation AssignReposToDept {
  assignRepoToDepartment(
    ou: "engineering"
    repositories: [
      "https://github.com/devplatform/core-api",
      "https://github.com/devplatform/infrastructure"
    ]
  ) {
    ou
    description
    repositories
  }
}
```

#### Step 5.2: Query Department Repositories
```graphql
query DepartmentRepos {
  department(ou: "engineering") {
    ou
    description
    repositories
    members {
      uid
      cn
      mail
    }
  }
}
```

#### Step 5.3: Dashboard Shows Combined Access
**User sees:**
1. Personal repositories (assigned directly)
2. Department repositories (from their department)
3. Grouped by source

---

### Phase 6: Live Gitea Data Integration

#### Step 6.1: Setup Required (Missing Pieces)

**A. Generate Gitea API Token**
1. Login to Gitea: `http://localhost:30009`
2. Settings → Applications → Generate New Token
3. Select scopes: `repo`, `admin:org`, `user`
4. Copy token

**B. Update Gitea Service Secret**
```yaml
# gitea-service/k8s/02-secret.yaml
stringData:
  GITEA_TOKEN: "your-real-token-here"
  JWT_SECRET: "your-super-secret-jwt-key-change-in-production"
```

**C. Apply and Restart**
```batch
kubectl apply -f gitea-service\k8s\02-secret.yaml
kubectl rollout restart deployment/gitea-service -n dev-platform
```

#### Step 6.2: Create Repositories in Gitea
1. Login to Gitea web UI
2. Create organization: `devplatform`
3. Create repositories:
   - `devplatform/backend`
   - `devplatform/frontend`
   - `devplatform/infrastructure`

#### Step 6.3: Query Live Repository Data
**From Frontend:**
```graphql
query GetLiveRepoData {
  searchRepositories(query: "devplatform", limit: 10) {
    id
    name
    fullName
    description
    htmlUrl
    cloneUrl
    stars
    forks
    size
    language
    updatedAt
    owner {
      login
      fullName
    }
  }
}
```

**Request to:** `http://localhost:30011/graphql`

**Display in Dashboard:**
- Repository cards with live data
- Star count, fork count
- Last update timestamp
- Programming language
- Clone button with URL

---

### Phase 7: Complete Dashboard Features

#### Feature 1: Repository Cards
```
┌─────────────────────────────────────────┐
│ 📦 devplatform/backend                  │
│                                         │
│ LDAP Manager Service in Go              │
│                                         │
│ ⭐ 5    🍴 2    Go    Updated 2h ago   │
│                                         │
│ [Clone] [View in Gitea]                 │
└─────────────────────────────────────────┘
```

#### Feature 2: User Profile Card
```
┌─────────────────────────────────────────┐
│ 👤 John Doe                             │
│                                         │
│ 📧 john.doe@devplatform.local           │
│ 🏢 Engineering Department               │
│ 🆔 UID: 10000 / GID: 10000             │
│                                         │
│ Repositories: 3 personal, 2 department  │
└─────────────────────────────────────────┘
```

#### Feature 3: Repository List with Filters
- Filter by department
- Filter by language
- Search by name
- Sort by stars, forks, or last update

#### Feature 4: Department Overview
```
┌─────────────────────────────────────────┐
│ 🏢 Engineering Department               │
│                                         │
│ 👥 Members: 5                           │
│ 📦 Repositories: 7                      │
│                                         │
│ Team Repositories:                      │
│ • devplatform/core-api                  │
│ • devplatform/infrastructure            │
│                                         │
│ Team Members:                           │
│ • John Doe (Tech Lead)                  │
│ • Jane Smith                            │
└─────────────────────────────────────────┘
```

---

## Implementation Steps Summary

### Immediate Next Steps

1. **Fix Gitea Service Health Check**
   - Update `client.go` to not require auth for health checks
   - OR generate real Gitea API token

2. **Create Test Repositories in Gitea**
   - Login to Gitea web UI
   - Create `devplatform` organization
   - Create 3-5 test repositories

3. **Update Frontend Dashboard**
   - Create Repository List component
   - Fetch user repositories from LDAP Manager
   - Display as cards with metadata

4. **Test Complete Flow**
   - Login as `john.doe`
   - Assign repositories via GraphQL
   - See repositories on dashboard
   - Verify data from both LDAP and Gitea

### Future Enhancements

5. **Add Gitea Service Integration**
   - Query live repo data from Gitea
   - Combine with LDAP assignments
   - Show enriched metadata

6. **Build Admin Panel**
   - User management UI
   - Repository assignment UI
   - Department management UI

7. **Add Repository Actions**
   - Clone repository (copy clone URL)
   - View in Gitea (direct link)
   - Fork repository
   - View commit history

8. **Implement Permissions**
   - Role-based access (admin vs user)
   - Department-based visibility
   - Repository access control

---

## Current Blockers

1. ⚠️ **Gitea Service** - 401 errors due to fake token
   - **Solution:** Generate real token OR fix health check

2. ⚠️ **No Test Repositories** - Gitea is empty
   - **Solution:** Create test repos in Gitea UI

3. ⚠️ **Frontend Dashboard** - Not built yet
   - **Solution:** Create repository list component

---

## Testing Workflow

### Test 1: Verify Istio Sidecars
```bash
# Check pods have 2/2 containers (app + istio-proxy)
kubectl get pods -n dev-platform

# Expected output:
# ldap-manager-xxx   2/2   Running
# gitea-service-xxx  2/2   Running
```

### Test 2: Verify mTLS
```bash
# Get pod name
POD_NAME=$(kubectl get pod -n dev-platform -l app=ldap-manager -o jsonpath='{.items[0].metadata.name}')

# Check mTLS status
istioctl authn tls-check $POD_NAME -n dev-platform

# Expected: STRICT mode for all connections
```

### Test 3: Test Authentication (via LoadBalancer)
```bash
# Test login via LoadBalancer port
curl -X POST http://localhost:30008/graphql \
  -H "Content-Type: application/json" \
  -d '{"query":"mutation { login(uid: \"john.doe\", password: \"password123\") { token user { uid repositories } } }"}'
```

### Test 4: Test Authentication (via Istio Gateway)
```bash
# Get Istio Ingress Gateway IP
INGRESS_HOST=$(kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
INGRESS_PORT=$(kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.spec.ports[?(@.name=="http2")].port}')

# Test via Istio Gateway with Host header
curl -X POST http://$INGRESS_HOST:$INGRESS_PORT/graphql \
  -H "Host: ldap-manager.localhost" \
  -H "Content-Type: application/json" \
  -d '{"query":"mutation { login(uid: \"john.doe\", password: \"password123\") { token } }"}'
```

### Test 5: Assign Repository
```bash
# Assign repos to user
curl -X POST http://localhost:30008/graphql \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <TOKEN>" \
  -d '{"query":"mutation { assignRepoToUser(uid: \"john.doe\", repositories: [\"https://github.com/test/repo1\"]) { uid repositories } }"}'
```

### Test 6: Query User Repos
```bash
# Get user with repos
curl -X POST http://localhost:30008/graphql \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <TOKEN>" \
  -d '{"query":"query { me { uid repositories } }"}'
```

### Test 7: Query Gitea (After Fix)
```bash
# Get repos from Gitea
curl -X POST http://localhost:30011/graphql \
  -H "Content-Type: application/json" \
  -d '{"query":"query { searchRepositories(query: \"devplatform\") { name fullName stars } }"}'
```

### Test 8: Verify Service Mesh Communication
```bash
# Check Envoy proxy logs to see mTLS connections
kubectl logs -n dev-platform -l app=ldap-manager -c istio-proxy --tail=20

# Check for successful mTLS handshakes and encrypted traffic
```

---

## Istio Observability

### View Service Mesh in Kiali
```bash
# Install Kiali (if not installed)
kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.20/samples/addons/kiali.yaml

# Access Kiali dashboard
istioctl dashboard kiali
```

**What you'll see:**
- Service topology graph
- Traffic flow between services
- Request rates and error rates
- mTLS status indicators
- Distributed traces

### Distributed Tracing with Jaeger
```bash
# Install Jaeger (if not installed)
kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.20/samples/addons/jaeger.yaml

# Access Jaeger UI
istioctl dashboard jaeger
```

**Trace a login request:**
1. User logs in via frontend
2. Request goes through Istio Ingress Gateway
3. Routes to ldap-manager service
4. ldap-manager queries OpenLDAP
5. Returns JWT token
6. See complete request trace with timing

### Metrics with Prometheus & Grafana
```bash
# Install Prometheus
kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.20/samples/addons/prometheus.yaml

# Install Grafana
kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.20/samples/addons/grafana.yaml

# Access Grafana
istioctl dashboard grafana
```

**Key Metrics:**
- Request rate (requests/second)
- Error rate (4xx, 5xx responses)
- Request duration (p50, p90, p99)
- Connection pool utilization
- Circuit breaker status

---

## Istio Security Features in Action

### 1. Automatic mTLS Between Services

**Without Istio (Plain HTTP):**
```
ldap-manager → OpenLDAP (port 389)
Plain text LDAP protocol
```

**With Istio:**
```
ldap-manager → Envoy Proxy (mTLS encrypt)
             → Network (encrypted)
             → Envoy Proxy (mTLS decrypt)
             → OpenLDAP
```

### 2. JWT Validation at Gateway

**Configuration:**
```yaml
# RequestAuthentication validates JWT tokens
apiVersion: security.istio.io/v1beta1
kind: RequestAuthentication
metadata:
  name: ldap-manager-jwt
spec:
  jwtRules:
  - issuer: "ldap-manager"
```

**How it works:**
1. User gets JWT from login mutation
2. Frontend sends JWT in Authorization header
3. Istio Gateway validates JWT before routing
4. Invalid/expired tokens rejected at edge
5. Application receives only valid requests

### 3. Authorization Policies

**Public endpoints (no auth required):**
- `/health` - Kubernetes liveness probe
- `/ready` - Kubernetes readiness probe

**Protected endpoints (JWT required):**
- `/graphql` - All queries/mutations except login

**Example Policy:**
```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
  name: ldap-manager-authz
spec:
  action: ALLOW
  rules:
  - to:
    - operation:
        paths: ["/health", "/ready"]
  - to:
    - operation:
        paths: ["/graphql"]
    from:
    - source:
        principals: ["cluster.local/ns/dev-platform/sa/*"]
```

### 4. Circuit Breaking

**Configuration in DestinationRule:**
```yaml
outlierDetection:
  consecutiveErrors: 5
  interval: 30s
  baseEjectionTime: 30s
  maxEjectionPercent: 50
```

**How it protects:**
- If a pod has 5 consecutive errors
- It's ejected from the load balancer pool for 30s
- Prevents cascading failures
- Automatic recovery after cooldown

---

## Deployment with Istio

### Deploy Services with Istio Sidecars

**For LDAP Manager:**
```batch
cd backend
deploy-istio.bat
```

**What happens:**
1. Builds Docker image
2. Loads into k8s.io namespace
3. Labels namespace: `istio-injection=enabled`
4. Deploys Kubernetes manifests
5. Istio automatically injects sidecar
6. Applies Istio configs (Gateway, VirtualService, etc.)
7. Pod starts with 2 containers: `ldap-manager` + `istio-proxy`

**For Gitea Service:**
```batch
cd gitea-service
deploy-istio.bat
```

### Verify Istio Injection
```bash
# Check pods have sidecars
kubectl get pods -n dev-platform -o custom-columns=\
NAME:.metadata.name,\
CONTAINERS:.spec.containers[*].name,\
READY:.status.containerStatuses[*].ready

# Expected output:
# NAME                    CONTAINERS                     READY
# ldap-manager-xxx        ldap-manager,istio-proxy      true,true
# gitea-service-xxx       gitea-service,istio-proxy     true,true
```

### View Istio Configurations
```bash
# List all Istio resources
kubectl get gateway,virtualservice,destinationrule,peerauthentication,authorizationpolicy -n dev-platform

# Describe a specific resource
kubectl describe gateway ldap-manager-gateway -n dev-platform
kubectl describe virtualservice ldap-manager-vs -n dev-platform
```

---

## Decision Points

**Question 1:** Should we fix Gitea Service health check first, or just generate a real token?
- **Option A:** Fix code to not require auth for `/version` endpoint
- **Option B:** Generate token from Gitea UI (2 minutes)

**Question 2:** How should repositories be displayed?
- **Option A:** Simple list from LDAP attribute (quick)
- **Option B:** Enriched cards with Gitea metadata (better UX)

**Question 3:** Where should the "Assign Repository" UI live?
- **Option A:** Admin-only panel
- **Option B:** User can request, admin approves
- **Option C:** Automatic based on department

---

## Next Action

**Choose your path:**

**Path A: Quick Win (30 minutes)**
1. Generate Gitea token
2. Update gitea-service secret
3. Create 2-3 test repos in Gitea
4. Build simple repository list in frontend

**Path B: Proper Fix (1 hour)**
1. Fix gitea-service health check code
2. Set up Gitea organization properly
3. Build full repository dashboard with cards
4. Add assignment UI

Which path would you like to take?
