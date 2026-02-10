# TeamVault Bug Fix Summary

**Date:** 2026-02-10  
**Fixed by:** OpenClaw Subagent (mega-fix-all-bugs)

---

## Build Status

| Target | Status |
|--------|--------|
| `docker compose build server` | ✅ PASS |
| `cd web && npm run build` | ✅ PASS |
| Curl verification tests | ✅ ALL PASS |

---

## Bugs Fixed

### 1. Frontend: Project UUID vs Name (BUG-2 — Critical)

**Files modified:**
- `web/src/app/projects/[id]/page.tsx`
- `web/src/app/projects/[id]/secrets/[path]/page.tsx`

**Problem:** The web UI used `project.id` (UUID) in API URLs for secrets (e.g., `GET /api/v1/secrets/a15f23a9-...`), but the server expects `project.name` (e.g., `GET /api/v1/secrets/my-project`). This broke ALL secret operations in the UI.

**Fix:** Changed both project page and secret detail page to:
1. First resolve the project UUID to a `Project` object by fetching all projects and filtering by `id`
2. Use `project.name` in all API calls to the secrets endpoints
3. Updated `loadSecrets()`, `handleCreate()`, `handleDelete()`, `handleUpdate()`, `handleSetRotation()`, `handleRotateNow()`, `handleCompareVersions()`, and `loadRotation()` to use project name instead of UUID

---

### 2. Project Creation 500 (BUG-1 — Critical)

**Root cause analysis:** The 500 error was observed during Playwright testing against a hardcoded external IP (`http://142.231.83.48:8443/api/v1`). The frontend audit already fixed this to use relative URLs (`/api/v1`) with Next.js proxy rewrites. The project creation handler itself works correctly — verified with curl tests (201 Created with both filled and empty description fields).

**Status:** Fixed by the frontend audit's prior changes (hardcoded IP removal + proxy rewrites). Verified working.

---

### 3. Org Creation 500 (BUG-3 — Critical)

**Same root cause as BUG-1.** The org creation handler works correctly. Verified with curl (201 Created).

**Status:** Fixed by the frontend audit's prior changes. Verified working.

---

### 4. CLI: List Commands JSON Parsing (CLI BUG-1 — Critical)

**Files modified:**
- `cmd/teamvault/cli/client.go` — `ListSecrets()`
- `cmd/teamvault/cli/org.go` — `ListOrgs()`
- `cmd/teamvault/cli/team.go` — `ListTeams()`
- `cmd/teamvault/cli/policy.go` — `ListPolicies()`

**Problem:** Server returns bare JSON arrays `[...]` but CLI tried to unmarshal into wrapper structs like `struct { Secrets []SecretListItem "json:\"secrets\"" }`.

**Fix:** Changed all list functions to unmarshal directly into `[]T` slices:
```go
// Before:
var resp struct { Secrets []SecretListItem `json:"secrets"` }
// After:
var resp []SecretListItem
```

Affected 4 files, 4 functions.

---

### 5. CLI: Team Create Wrong Endpoint (CLI BUG-2 — Critical)

**File modified:** `cmd/teamvault/cli/team.go`

**Problem:** `CreateTeam()` POSTed to `/api/v1/teams` but server route is `POST /api/v1/orgs/{id}/teams`.

**Fix:**
1. Changed URL to `fmt.Sprintf("/api/v1/orgs/%s/teams", orgID)`
2. Removed `org_id` from request body (it's in the URL now)
3. Added org name-to-ID resolution in `runTeamCreate()` — fetches org list and matches by name or ID

---

### 6. CLI: Doctor Health Path (CLI BUG-3 — Medium)

**File modified:** `cmd/teamvault/cli/doctor.go`

**Problem:** `Health()` queried `/api/v1/health` and `checkServerReachable()` used `/api/v1/health`, but server health endpoint is at `/health` (root, no `/api/v1` prefix).

**Fix:** Changed both endpoints from `/api/v1/health` to `/health`.

---

### 7. Security: MaxBytesReader (FAIL-3 — Medium)

**File modified:** `internal/api/helpers.go`

**Problem:** `decodeJSON()` had no request body size limit. Attackers could send multi-GB payloads to exhaust server memory.

**Fix:** Added `http.MaxBytesReader(nil, r.Body, 1<<20)` (1 MiB limit) before JSON decoding. Large payloads now return 400 Bad Request ("invalid request body") instead of being processed.

**Verified:** 2MB payload returns HTTP 400.

---

### 8. Security: X-Forwarded-For Spoofing (FAIL-4 — High)

**File modified:** `internal/api/middleware.go`

**Problem:** `clientIP()` unconditionally trusted `X-Forwarded-For` and `X-Real-IP` headers, enabling:
- Rate limit bypass (each spoofed IP gets its own bucket)
- Audit log spoofing (forged source IPs in logs)

**Fix:** Removed `X-Forwarded-For` and `X-Real-IP` header checks. Now uses only `r.RemoteAddr`. Added documentation comment about configuring trusted proxy headers if running behind a reverse proxy.

---

### 9. Security: Missing Security Headers (FAIL — High)

**Files modified:**
- `internal/api/middleware.go` — Added `securityHeadersMiddleware()`
- `internal/api/server.go` — Added to handler chain

**Problem:** No security headers were set on responses.

**Fix:** Added new `securityHeadersMiddleware` that sets:
| Header | Value |
|--------|-------|
| `X-Content-Type-Options` | `nosniff` |
| `X-Frame-Options` | `DENY` |
| `X-XSS-Protection` | `1; mode=block` |
| `Strict-Transport-Security` | `max-age=63072000; includeSubDomains` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |

**Verified:** All 5 headers present on every response.

---

### 10. Security: Restrict Project/Secret/Org Listing (FAIL — Critical)

**Files modified:**
- `internal/api/project_handlers.go` — `handleListProjects()`
- `internal/api/org_handlers.go` — `handleListOrgs()`
- `internal/api/secret_handlers.go` — `handleListSecrets()`
- `internal/api/server.go` — Audit endpoint now admin-only

**Problem:** Any authenticated user could enumerate ALL projects, organizations, secrets, and audit events, regardless of ownership.

**Fix:**
1. **Project listing:** Non-admin users now only see projects they created (`CreatedBy == UserID`)
2. **Org listing:** Non-admin users now only see orgs they created
3. **Secret listing:** Added policy evaluation (read permission on `project/*`). Falls back to ownership check — project creator or admin can list
4. **Audit log:** Wrapped with `adminOnly()` middleware — only admins can access audit events

---

## Files Changed Summary

| File | Changes |
|------|---------|
| `web/src/app/projects/[id]/page.tsx` | UUID→name for secret API calls |
| `web/src/app/projects/[id]/secrets/[path]/page.tsx` | UUID→name for all secret/rotation API calls |
| `cmd/teamvault/cli/client.go` | `ListSecrets`: bare array parsing |
| `cmd/teamvault/cli/org.go` | `ListOrgs`: bare array parsing |
| `cmd/teamvault/cli/team.go` | `ListTeams`: bare array parsing; `CreateTeam`: correct endpoint + org resolution |
| `cmd/teamvault/cli/policy.go` | `ListPolicies`: bare array parsing |
| `cmd/teamvault/cli/doctor.go` | Health endpoint path: `/api/v1/health` → `/health` |
| `internal/api/helpers.go` | Added `MaxBytesReader` (1MB limit) |
| `internal/api/middleware.go` | Removed X-Forwarded-For trust; added security headers middleware |
| `internal/api/server.go` | Added security headers to chain; audit log now admin-only |
| `internal/api/project_handlers.go` | Ownership filtering on list |
| `internal/api/org_handlers.go` | Ownership filtering on list |
| `internal/api/secret_handlers.go` | Policy check + ownership check on list |

**Total: 13 files modified across frontend, CLI, and server**
