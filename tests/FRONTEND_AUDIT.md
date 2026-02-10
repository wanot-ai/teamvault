# TeamVault Frontend Audit Report

**Date:** 2026-02-10  
**Auditor:** QA Subagent  
**Scope:** `web/src/` — API calls, hardcoded URLs, error handling, loading/empty states  
**Build Status:** ✅ PASS (`npm run build` succeeds)

---

## Summary

| Category | Issues Found | Fixed |
|----------|-------------|-------|
| Hardcoded URLs | 1 | 1 |
| API URL mismatches | 10 | 10 |
| Missing error handling | 0 | — |
| Missing loading/empty states | 0 | — |
| Configuration issues | 1 | 1 |
| **Total** | **12** | **12** |

---

## Issues Found & Fixed

### 🔴 CRITICAL — Hardcoded External IP in API_BASE

**File:** `src/lib/api.ts:3`  
**Before:** `const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://142.231.83.48:8443/api/v1";`  
**After:** `const API_BASE = process.env.NEXT_PUBLIC_API_URL || "/api/v1";`  
**Impact:** All API calls would fail in any environment except the original dev machine. Hardcoded IP also leaks infrastructure details.

---

### 🔴 CRITICAL — Secret Versions Endpoint Mismatch

**File:** `src/lib/api.ts` — `secrets.versions()`  
**Frontend called:** `GET /api/v1/secrets/{project}/{path}/versions`  
**Server expects:** `GET /api/v1/secret-versions/{project}/{path}`  
**Fix:** Changed path to `/secret-versions/${projectId}/${path}`

---

### 🔴 CRITICAL — Health/Ready Endpoints Under Wrong Prefix

**File:** `src/lib/api.ts` — `health.check()`, `health.ready()`  
**Frontend called:** `GET /api/v1/health`, `GET /api/v1/ready`  
**Server expects:** `GET /health`, `GET /ready` (mounted at root, not under `/api/v1`)  
**Fix:** Changed to use `fetch("/health")` and `fetch("/ready")` directly, bypassing the API_BASE prefix.

---

### 🔴 CRITICAL — Team Members Endpoint Mismatch

**File:** `src/lib/api.ts` — `teamMembers.*`  
**Frontend called:** `/orgs/{orgId}/teams/{teamId}/members` and `/orgs/{orgId}/teams/{teamId}/members/{userId}`  
**Server expects:** `/teams/{id}/members` (POST/DELETE/GET with no org nesting, DELETE uses body not URL param)  
**Fix:** Updated all teamMembers methods to use `/teams/{teamId}/members`. Changed `remove()` to send `{ user_id }` in DELETE body.

---

### 🔴 CRITICAL — Agents Endpoint Mismatch

**File:** `src/lib/api.ts` — `agents.*`  
**Frontend called:** `/orgs/{orgId}/teams/{teamId}/agents/{agentId}` (nested under orgs)  
**Server expects:** 
- `POST/GET /teams/{id}/agents` (list/create)
- `GET/DELETE /agents/{agentId}` (individual agent ops)

**Fix:** Updated `list`/`create` to use `/teams/{teamId}/agents`, `get`/`delete`/`revoke` to use `/agents/{agentId}`.

---

### 🔴 CRITICAL — Lease Issue Endpoint Mismatch

**File:** `src/lib/api.ts` — `leases.issue()`  
**Frontend called:** `POST /api/v1/leases`  
**Server expects:** `POST /api/v1/lease/database` (type-specific path)  
**Fix:** Changed to `POST /lease/${data.type}` to dynamically use the lease type in the URL.

---

### 🟡 HIGH — Lease Revoke Endpoint Mismatch

**File:** `src/lib/api.ts` — `leases.revoke()`  
**Frontend called:** `POST /api/v1/leases/{id}/revoke`  
**Server expects:** `POST /api/v1/lease/{id}/revoke` (singular `lease`, not plural)  
**Fix:** Changed to `/lease/${leaseId}/revoke`.

---

### 🟡 HIGH — Rotation API Endpoint Mismatch

**File:** `src/lib/api.ts` — `rotation.*`  
**Frontend called:** `GET/PUT/DELETE /secrets/{project}/{path}/rotation`, `POST /secrets/{project}/{path}/rotate`  
**Server expects:** `GET /rotation-status/{project}/{path}` (read-only; no PUT/DELETE/POST rotate)  
**Fix:** Changed `get()` to use `/rotation-status/...`. Updated `rotateNow()` to use the secrets POST handler (`POST /secrets/{project}/{path}` with `action: "rotate"` body). Added comments noting set/delete are not yet server-side.

---

### 🟡 HIGH — IAM Policy Update Uses Wrong HTTP Method

**File:** `src/lib/api.ts` — `iamPolicies.update()`, `iamPolicies.toggle()`  
**Frontend used:** `PATCH`  
**Server expects:** `PUT /api/v1/iam-policies/{id}`  
**Fix:** Changed both `update()` and `toggle()` to use `PUT`.

---

### 🟡 HIGH — OIDC Callback Method Mismatch

**File:** `src/lib/api.ts` — `oidc.callback()`  
**Frontend used:** `POST /auth/oidc/callback` with JSON body `{ token }`  
**Server expects:** `GET /auth/oidc/callback` with query params (standard OAuth2 redirect)  
**Fix:** Changed to GET with `?code=` query parameter. Also updated `login/page.tsx` to check for both `?code=` and `?token=` search params.

---

### 🟡 MEDIUM — Teams GET Individual Endpoint Missing

**File:** `src/lib/api.ts` — `teams.get()`  
**Frontend called:** `GET /orgs/{orgId}/teams/{teamId}`  
**Server reality:** No individual team GET endpoint exists  
**Fix:** Implemented workaround — fetches all teams for the org via `GET /orgs/{orgId}/teams` and filters by ID client-side.

---

### 🟢 LOW — Missing Next.js Proxy Configuration

**File:** `next.config.ts`  
**Issue:** No proxy rewrites configured, so relative `/api/v1` URLs wouldn't reach the backend during development.  
**Fix:** Added `rewrites()` rules to proxy `/api/v1/*`, `/health`, and `/ready` to `http://localhost:8443` (configurable via `BACKEND_URL` env var).

---

## Endpoints Not Yet Implemented Server-Side

These API calls exist in the frontend but have **no matching server routes**. They will return 404/405:

| Frontend Method | Expected Route | Status |
|----------------|---------------|--------|
| `orgs.update()` | `PATCH /orgs/{id}` | ❌ Not implemented |
| `orgs.delete()` | `DELETE /orgs/{id}` | ❌ Not implemented |
| `teams.update()` | `PATCH /orgs/{id}/teams/{teamId}` | ❌ Not implemented |
| `teams.delete()` | `DELETE /orgs/{id}/teams/{teamId}` | ❌ Not implemented |
| `rotation.set()` | `PUT /rotation-status/{project}/{path}` | ❌ Not implemented |
| `rotation.delete()` | `DELETE /rotation-status/{project}/{path}` | ❌ Not implemented |

These are documented with comments in the code. Frontend gracefully handles errors via toast notifications.

---

## Error Handling Assessment

✅ **All pages properly handle API errors** — every `async` call is wrapped in try/catch with `toast.error()` and `console.error()`.

✅ **Auth context** handles token refresh failures by logging out.

✅ **401 responses** are globally handled in `apiFetch()` — clears token, redirects to `/login`.

✅ **204 No Content** responses are handled properly.

---

## Loading & Empty States Assessment

| Page | Loading State | Empty State |
|------|:---:|:---:|
| Dashboard | ✅ Spinner | ✅ "No projects yet" CTA |
| Project Secrets | ✅ Spinner | ✅ "No secrets yet" CTA |
| Secret Detail | ✅ Spinner | ✅ "Secret not found" |
| Leases | ✅ Spinner | ✅ "No leases" CTA |
| Organizations | ✅ Spinner | ✅ "No organizations yet" CTA |
| Org Detail (Teams) | ✅ Spinner | ✅ "No teams yet" CTA |
| Team Detail | ✅ Full-page spinner | ✅ "No members/agents" per tab |
| IAM Policies | ✅ Spinner per tab | ✅ "No policies" per type CTA |
| Audit Log | ✅ Spinner | ✅ "No audit events" with filter hint |
| Settings (SA/Policies) | ✅ Spinner per tab | ✅ "No service accounts/policies" |
| Login | ✅ SSO loading state | N/A |

All pages have proper loading spinners and meaningful empty states with call-to-action buttons.

---

## Files Modified

1. `src/lib/api.ts` — 10 endpoint fixes + hardcoded IP removal
2. `src/app/login/page.tsx` — OIDC callback param fix
3. `next.config.ts` — Added proxy rewrites for development
