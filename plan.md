# Plan: Silent Token Refresh + Auth Guards

## Problem
- Keycloak access tokens expire in ~5 minutes
- The `refresh_token` from Keycloak is received but **discarded**
- No 401 handling — expired tokens just cause API errors
- No route protection — anyone can navigate to `/dashboard` without auth
- Logout bug: removes wrong localStorage key (`"token"` instead of `"authToken"`)

## Approach: Centralized Auth Context + Silent Refresh

No new dependencies. Pure React context + existing Keycloak ROPC refresh_token grant.

## Files to create/modify

### 1. Create `src/context/AuthContext.tsx` (NEW)
- React context + provider wrapping the entire app
- Stores: `accessToken`, `refreshToken`, `expiresAt`, `user`, `isAuthenticated`
- On mount: reads tokens from localStorage, checks if expired, attempts refresh if needed
- `login()`: calls Keycloak, stores **both** access_token and refresh_token + expires_at
- `logout()`: clears all auth state + localStorage, navigates to `/login`
- `refreshAccessToken()`: calls Keycloak with `grant_type=refresh_token`, updates tokens
- `scheduleRefresh()`: sets a timeout to refresh 60s before expiry (proactive, no jank)
- Exports `useAuth()` hook

### 2. Create `src/components/auth/RequireAuth.tsx` (NEW)
- Wrapper component that checks `isAuthenticated` from AuthContext
- If not authenticated → redirect to `/login`
- Simple: `if (!isAuthenticated) return <Navigate to="/login" />`

### 3. Modify `src/services/graphqlRequest.ts`
- Import `getValidToken()` from AuthContext (exported utility)
- Replace `localStorage.getItem("authToken")` with `getValidToken()` which:
  - Returns current token if still valid
  - Calls refresh if about to expire
  - Throws if refresh fails (triggers logout)

### 4. Modify `src/services/codeserverService.ts`
- Same change: use `getValidToken()` instead of direct localStorage read

### 5. Modify `src/services/userService.ts`
- `login()`: return the full Keycloak response (access_token, refresh_token, expires_in)
- Remove `logout()` from here (moved to AuthContext)
- Remove dead `register()` function

### 6. Modify `src/pages/auth/login/login.tsx`
- Use `useAuth().login()` instead of calling userService directly
- Remove direct localStorage writes (AuthContext handles it)

### 7. Modify `src/App.tsx`
- Wrap Router with `<AuthProvider>`
- Wrap protected routes with `<RequireAuth>`

### 8. Modify `src/components/header/header.tsx`
- Use `useAuth().logout()` instead of importing from userService

## Token Refresh Flow

```
Login → Keycloak returns access_token (5min) + refresh_token (30min)
  → Store both in localStorage + context
  → Schedule refresh at (expires_in - 60s) = ~4 minutes

Timer fires at 4 min mark:
  → POST to Keycloak token endpoint with grant_type=refresh_token
  → Get new access_token + new refresh_token
  → Update localStorage + context
  → Schedule next refresh

If refresh fails (refresh_token also expired):
  → Clear everything → redirect to /login
```

## localStorage Keys

| Key | Value |
|-----|-------|
| `authToken` | Current access_token (kept for backward compat) |
| `refreshToken` | Keycloak refresh_token |
| `tokenExpiresAt` | Unix timestamp (ms) when access_token expires |
| `user` | JSON user object |

## Order of Implementation

1. `AuthContext.tsx` — core logic
2. `RequireAuth.tsx` — route guard
3. `userService.ts` — return full token response
4. `login.tsx` — use AuthContext
5. `graphqlRequest.ts` — use getValidToken
6. `codeserverService.ts` — use getValidToken
7. `App.tsx` — wire provider + guards
8. `header.tsx` — use AuthContext logout
