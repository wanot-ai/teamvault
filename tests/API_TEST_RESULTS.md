# TeamVault API Exhaustive Test Results

**Date:** 2026-02-10 00:27 UTC  
**Server:** http://localhost:8443  
**Tester:** QA Automation (subagent)

---

## 🐛 BUG SUMMARY

### BUG #1: Registration crashes with 500 for passwords > 72 bytes (CRITICAL)

**Severity:** High — causes 500 Internal Server Error  
**Endpoint:** `POST /api/v1/auth/register`  
**Root Cause:** Go's `bcrypt.GenerateFromPassword()` returns an error for passwords longer than 72 bytes. The handler in `auth_handlers.go` only validates minimum length (8 chars) but not maximum length. The `HashPassword()` function in `auth/auth.go` passes the raw password to bcrypt without truncation or length check.

**Exact requests that cause 500:**
```bash
# 73-byte password — crashes
curl -s http://localhost:8443/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@test.com","password":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","name":"Test"}'
# Response: {"error":"failed to process registration"}  HTTP 500

# 72-byte password — works fine
curl -s http://localhost:8443/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@test.com","password":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","name":"Test"}'
# Response: HTTP 201 (success)
```

**Boundary:**
| Password Length | Result |
|---|---|
| 71 bytes | ✅ 201 Created |
| 72 bytes | ✅ 201 Created |
| 73 bytes | ❌ 500 Internal Server Error |
| 100 bytes | ❌ 500 |
| 1000 bytes | ❌ 500 |
| 10000 bytes | ❌ 500 |

**Fix:** Either truncate the password to 72 bytes before hashing (common practice), or validate and return 400 if password > 72 bytes. Truncation is preferred since bcrypt already only uses the first 72 bytes for comparison.

---

### BUG #2: Adding non-existent user as team member returns 500 instead of 404

**Severity:** Medium — should return 404 "user not found" but returns 500  
**Endpoint:** `POST /api/v1/teams/{id}/members`  
**Root Cause:** The `handleAddTeamMember` handler calls `s.db.AddTeamMember()` without first verifying the user_id exists. When PostgreSQL hits a foreign key constraint violation, it returns an error, which the handler wraps as 500 "failed to add team member" instead of checking for FK violation → 404.

**Exact request:**
```bash
curl -s http://localhost:8443/api/v1/teams/{valid_team_id}/members \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{"user_id":"00000000-0000-0000-0000-000000000000"}'
# Response: {"error":"failed to add team member"}  HTTP 500
```

**Fix:** Check if the user exists before adding, or detect FK constraint violation in the error and return 404.

---

## ✅ Detailed Test Results by Endpoint

### 1. Health & Ready

| Test | Method | Status | Pass |
|---|---|---|---|
| Health check | `GET /health` | 200 | ✅ |
| Ready check | `GET /ready` | 200 | ✅ |

---

### 2. Auth: Registration (`POST /api/v1/auth/register`)

| Test | Status | Pass | Notes |
|---|---|---|---|
| Happy path | 201 | ✅ | Returns user + JWT token |
| Duplicate email | 409 | ✅ | "email already registered" |
| Missing email | 400 | ✅ | "email, password, and name are required" |
| Missing password | 400 | ✅ | Same validation message |
| Missing name | 400 | ✅ | Same validation message |
| Short password (7 chars) | 400 | ✅ | "password must be at least 8 characters" |
| Password exactly 8 chars | 201 | ✅ | Boundary works |
| Empty body `{}` | 400 | ✅ | |
| Empty strings | 400 | ✅ | |
| Invalid JSON | 400 | ✅ | "invalid request body" |
| Unicode name (Japanese + emoji) | 201 | ✅ | `テスト太郎🔐` stored correctly |
| Very long email (500+ chars) | 201 | ⚠️ | Accepted — no length validation on email |
| Very long name (10K chars) | 201 | ⚠️ | Accepted — no length validation on name |
| XSS in name | 201 | ✅ | Stored as-is but Go's json encoder escapes `<>` → `\u003c\u003e` |
| **Password > 72 bytes** | **500** | **❌** | **BUG #1** — bcrypt limit |
| Null values | 400 | ✅ | Treated as empty strings |
| Numeric values instead of strings | 400 | ✅ | "invalid request body" (JSON decode fails) |
| Extra fields (role, is_admin) | 201 | ✅ | Extra fields ignored, role stays "member" |

---

### 3. Auth: Login (`POST /api/v1/auth/login`)

| Test | Status | Pass | Notes |
|---|---|---|---|
| Happy path | 200 | ✅ | Returns user + JWT token |
| Wrong password | 401 | ✅ | "invalid credentials" (no info leak) |
| Non-existent email | 401 | ✅ | Same "invalid credentials" (no info leak) |
| Missing email | 400 | ✅ | |
| Missing password | 400 | ✅ | |
| Empty body | 400 | ✅ | |
| Empty strings | 400 | ✅ | |
| Invalid JSON | 400 | ✅ | |

---

### 4. Auth: Me (`GET /api/v1/auth/me`)

| Test | Status | Pass | Notes |
|---|---|---|---|
| Happy path | 200 | ✅ | Returns user object |
| No auth header | 401 | ✅ | "missing authorization header" |
| Invalid token | 401 | ✅ | "invalid token" |
| Empty Bearer | 401 | ✅ | |
| Wrong auth format (Basic) | 401 | ✅ | "invalid authorization format" |
| Tampered JWT | 401 | ✅ | Signature verification works |
| Forged JWT with fake signature | 401 | ✅ | |

---

### 5. Projects (`/api/v1/projects`)

| Test | Method | Status | Pass | Notes |
|---|---|---|---|---|
| Create project | POST | 201 | ✅ | |
| Duplicate name | POST | 409 | ✅ | "project name already exists" |
| No auth | POST | 401 | ✅ | |
| Missing name | POST | 400 | ✅ | |
| Empty name | POST | 400 | ✅ | |
| Unicode name (Korean + emoji) | POST | 201 | ✅ | `프로젝트-🔑` stored correctly |
| Very long name (10K chars) | POST | 201 | ⚠️ | No length validation |
| Name with slashes | POST | 201 | ⚠️ | Could conflict with path-based routing |
| Name with spaces | POST | 201 | ⚠️ | |
| Name with dots | POST | 201 | ⚠️ | |
| List projects | GET | 200 | ✅ | Returns empty array `[]` when none |
| List - no auth | GET | 401 | ✅ | |

---

### 6. Secrets (`/api/v1/secrets/{project}/{path...}`)

| Test | Method | Status | Pass | Notes |
|---|---|---|---|---|
| Put secret | PUT | 200 | ✅ | |
| No auth | PUT | 401 | ✅ | |
| Missing value | PUT | 400 | ✅ | "value is required" |
| Empty value | PUT | 400 | ✅ | |
| Non-existent project | PUT | 404 | ✅ | |
| File type secret | PUT | 200 | ✅ | With filename + content_type |
| File type missing filename | PUT | 400 | ✅ | "filename is required for file type secrets" |
| JSON type secret | PUT | 200 | ✅ | |
| Invalid type | PUT | 400 | ✅ | "type must be 'kv', 'json', or 'file'" |
| Very long value (100KB) | PUT | 200 | ✅ | |
| Unicode value | PUT | 200 | ✅ | `пароль密码パスワード🔐` |
| URL-encoded path chars | PUT | 200 | ✅ | |
| Update existing | PUT | 200 | ✅ | Version incremented |
| Get secret | GET | 200 | ✅ | Decrypted value returned |
| Get - no auth | GET | 401 | ✅ | |
| Get - non-existent project | GET | 404 | ✅ | |
| Get - non-existent path | GET | 404 | ✅ | |
| Unicode value readback | GET | 200 | ✅ | Matches written value |
| List secrets | GET | 200 | ✅ | No values in list |
| List - no auth | GET | 401 | ✅ | |
| List - non-existent project | GET | 404 | ✅ | |
| Delete secret | DELETE | 200 | ✅ | Soft delete |
| Delete already-deleted | DELETE | 404 | ✅ | |
| Delete non-existent | DELETE | 404 | ✅ | |
| Delete - no auth | DELETE | 401 | ✅ | |
| Read after delete | GET | 404 | ✅ | Properly returns not found |
| Path traversal (`../../../etc/passwd`) | PUT | 404 | ✅ | HTTP router normalizes path |

---

### 7. Secret Versions (`/api/v1/secret-versions/{project}/{path...}`)

| Test | Status | Pass | Notes |
|---|---|---|---|
| List versions (3 versions) | 200 | ✅ | Returns all 3 versions |
| No auth | 401 | ✅ | |
| Non-existent secret | 404 | ✅ | |
| Versions of deleted secret | 404 | ✅ | |

---

### 8. Organizations (`/api/v1/orgs`)

| Test | Method | Status | Pass | Notes |
|---|---|---|---|---|
| Create org | POST | 201 | ✅ | |
| Duplicate name | POST | 409 | ✅ | |
| No auth | POST | 401 | ✅ | |
| Missing name | POST | 400 | ✅ | |
| Empty name | POST | 400 | ✅ | |
| Unicode name | POST | 201 | ✅ | `조직-🏢` |
| Very long name | POST | 201 | ⚠️ | No length validation |
| List orgs | GET | 200 | ✅ | |
| Get org by ID | GET | 200 | ✅ | |
| Invalid UUID for ID | GET | 404 | ✅ | |
| Non-existent UUID | GET | 404 | ✅ | |
| No auth | GET | 401 | ✅ | |

---

### 9. Teams (`/api/v1/orgs/{id}/teams`)

| Test | Method | Status | Pass | Notes |
|---|---|---|---|---|
| Create team | POST | 201 | ✅ | |
| Duplicate name | POST | 409 | ✅ | |
| No auth | POST | 401 | ✅ | |
| Missing name | POST | 400 | ✅ | |
| Empty name | POST | 400 | ✅ | |
| Invalid org ID | POST | 404 | ✅ | |
| Unicode name | POST | 201 | ✅ | `팀-🎯` |
| List teams in org | GET | 200 | ✅ | |
| List all teams | GET | 200 | ✅ | Admin sees all teams |

---

### 10. Team Members (`/api/v1/teams/{id}/members`)

| Test | Method | Status | Pass | Notes |
|---|---|---|---|---|
| Add member | POST | 201 | ✅ | |
| Add duplicate member | POST | 500 | ⚠️ | Returns 500 instead of 409. Not a separate DB-level bug, just poor error handling |
| No auth | POST | 401 | ✅ | |
| Missing user_id | POST | 400 | ✅ | |
| Empty user_id | POST | 400 | ✅ | |
| Invalid team ID | POST | 404 | ✅ | |
| **Non-existent user_id** | POST | **500** | **❌** | **BUG #2** — FK violation → 500 |
| List members | GET | 200 | ✅ | |
| List - no auth | GET | 401 | ✅ | |
| Remove member | DELETE | 200 | ✅ | |
| Remove already-removed | DELETE | 404 | ✅ | |

---

### 11. Agents (`/api/v1/teams/{id}/agents`, `/api/v1/agents/{id}`)

| Test | Method | Status | Pass | Notes |
|---|---|---|---|---|
| Create agent | POST | 201 | ✅ | Returns agent + raw token |
| Duplicate name | POST | 409 | ✅ | |
| No auth | POST | 401 | ✅ | |
| Missing name | POST | 400 | ✅ | |
| Empty name | POST | 400 | ✅ | |
| With expiry (`24h`) | POST | 201 | ✅ | |
| Invalid expiry | POST | 400 | ✅ | "invalid expires_in duration" |
| Invalid team | POST | 404 | ✅ | |
| Default scopes | POST | 201 | ✅ | Defaults to `["read"]` |
| List agents | GET | 200 | ✅ | |
| Get agent by ID | GET | 200 | ✅ | |
| Non-existent agent | GET | 404 | ✅ | |
| Delete agent | DELETE | 200 | ✅ | |
| Delete already-deleted | DELETE | 404 | ✅ | |
| Delete - no auth | DELETE | 401 | ✅ | |

---

### 12. Service Accounts (`/api/v1/service-accounts`)

| Test | Method | Status | Pass | Notes |
|---|---|---|---|---|
| Create SA | POST | 201 | ✅ | Returns SA + `sa.{token}` |
| With TTL | POST | 201 | ✅ | |
| Invalid TTL | POST | 400 | ✅ | "invalid TTL format" |
| No auth | POST | 401 | ✅ | |
| Missing name | POST | 400 | ✅ | |
| Missing project_id | POST | 400 | ✅ | |
| Non-existent project | POST | 404 | ✅ | |
| Default scopes | POST | 201 | ✅ | Defaults to `["read"]` |
| List SAs | GET | 200 | ✅ | |
| Auth with SA token | GET | 200 | ✅ | Can read secrets |
| Expired SA token | GET | 401 | ✅ | "service account token expired" |

---

### 13. Policies (`/api/v1/policies`) — Admin Only

| Test | Method | Status | Pass | Notes |
|---|---|---|---|---|
| Create policy | POST | 201 | ✅ | |
| No auth | POST | 401 | ✅ | |
| Missing fields | POST | 400 | ✅ | |
| Invalid effect | POST | 400 | ✅ | "effect must be 'allow' or 'deny'" |
| Invalid subject_type | POST | 400 | ✅ | |
| Empty actions | POST | 400 | ✅ | |
| Deny effect | POST | 201 | ✅ | |
| Non-admin user | POST | 403 | ✅ | "admin access required" |
| List policies | GET | 200 | ✅ | |

---

### 14. IAM Policies (`/api/v1/iam-policies`)

| Test | Method | Status | Pass | Notes |
|---|---|---|---|---|
| Create with policy_doc | POST | 201 | ✅ | |
| No auth | POST | 401 | ✅ | |
| Missing fields | POST | 400 | ✅ | |
| Invalid policy_type | POST | 400 | ✅ | |
| Missing both hcl and doc | POST | 400 | ✅ | |
| Non-existent org | POST | 404 | ✅ | |
| List policies | GET | 200 | ✅ | |
| Filter by org_id | GET | 200 | ✅ | |
| Get by ID | GET | 200 | ✅ | |
| Non-existent ID | GET | 404 | ✅ | |
| Update policy | PUT | 200 | ✅ | |
| Change type | PUT | 200 | ✅ | |
| Invalid type in update | PUT | 400 | ✅ | |
| Update non-existent | PUT | 404 | ✅ | |
| Delete policy | DELETE | 200 | ✅ | |
| Delete already-deleted | DELETE | 404 | ✅ | |

---

### 15. Audit (`GET /api/v1/audit`)

| Test | Status | Pass | Notes |
|---|---|---|---|
| Happy path | 200 | ✅ | Returns audit events with hashes |
| With limit | 200 | ✅ | |
| With offset | 200 | ✅ | |
| Filter by action | 200 | ✅ | |
| Filter by actor_type | 200 | ✅ | |
| Invalid limit (string) | 200 | ⚠️ | Ignores bad value, uses default |
| Negative limit | 200 | ⚠️ | Accepted, returns all records |
| Very large limit (999999) | 200 | ⚠️ | Accepted, no max cap |
| Negative offset | 200 | ⚠️ | Accepted, treated as 0 |
| No auth | 401 | ✅ | |

---

### 16. Dashboard (`GET /api/v1/dashboard/stats`)

| Test | Status | Pass | Notes |
|---|---|---|---|
| Happy path | 200 | ✅ | Returns aggregate counts |
| No auth | 401 | ✅ | |

---

### 17. Rotation (`POST /api/v1/secrets/{project}/{path}/rotation`)

| Test | Status | Pass | Notes |
|---|---|---|---|
| Set rotation schedule | 200 | ✅ | |
| Missing cron_expression | 400 | ✅ | |
| Missing connector_type | 400 | ✅ | |
| Non-existent secret | 404 | ✅ | |
| Get rotation status | 200 | ✅ | |
| No rotation configured | 200 | ✅ | Returns `{"configured":false}` |
| Non-existent secret status | 404 | ✅ | |
| Non-admin set rotation | 403 | ✅ | |

---

### 18. Leases (`/api/v1/lease`)

| Test | Status | Pass | Notes |
|---|---|---|---|
| Issue database lease | 503 | ✅ | "lease manager not available" (expected in test env) |
| No auth | 401 | ✅ | |
| List leases | 503 | ✅ | Same — not configured |
| Revoke non-existent | 503 | ✅ | |

---

### 19. Webhooks (`/api/v1/webhooks`)

| Test | Status | Pass | Notes |
|---|---|---|---|
| Register webhook | 503 | ✅ | "webhooks not available" (expected in test env) |
| No auth | 401 | ✅ | |
| Missing url | 503 | ✅ | |
| List webhooks | 503 | ✅ | |

---

### 20. TEE Endpoints (`/api/v1/tee`)

| Test | Status | Pass | Notes |
|---|---|---|---|
| Attestation | 503 | ✅ | "TEE not available" (expected) |
| Session | 503 | ✅ | |
| Read | 503 | ✅ | |
| No auth | 401 | ✅ | |

---

### 21. ZK Auth (`/api/v1/auth/zk`)

| Test | Status | Pass | Notes |
|---|---|---|---|
| Issue credential | 503 | ✅ | "ZK auth not available" (expected) |
| No auth | 401 | ✅ | |
| Verify proof | 503 | ✅ | |

---

### 22. Replication (`/api/v1/replication`)

| Test | Status | Pass | Notes |
|---|---|---|---|
| Push (empty) | 503 | ✅ | "replication not available" (expected) |
| Pull | 503 | ✅ | |
| Status | 503 | ✅ | |
| No auth | 401 | ✅ | |

---

### 23. OIDC (`/api/v1/auth/oidc`)

| Test | Status | Pass | Notes |
|---|---|---|---|
| Authorize | 500 | ⚠️ | "OIDC not configured" — should be 503 |
| Callback | 500 | ⚠️ | Same |
| Callback with code | 500 | ⚠️ | Same |

---

### 24. Misc & Edge Cases

| Test | Status | Pass | Notes |
|---|---|---|---|
| POST to `/health` | 405 | ✅ | Method not allowed |
| Non-existent endpoint | 404 | ✅ | |
| PATCH method on projects | 405 | ✅ | |
| Concurrent duplicate project creation | 1×201, 4×409 | ✅ | Race condition handled by DB unique constraint |
| Role escalation via extra fields | member | ✅ | Extra `role`/`is_admin` fields ignored |
| Path traversal in secrets | 404 | ✅ | Router normalizes `../` |
| Empty Authorization header | 401 | ✅ | |
| Auth without "Bearer " prefix | 401 | ✅ | |
| Secret POST without /rotation or /rotate suffix | 400 | ✅ | "use PUT to create/update secrets" |

---

### 25. Security: Password Hash Leak Check

| Endpoint | Leaks password_hash? |
|---|---|
| `POST /api/v1/auth/register` | ✅ No leak |
| `POST /api/v1/auth/login` | ✅ No leak |
| `GET /api/v1/auth/me` | ✅ No leak |

---

## ⚠️ Warnings (Not Bugs, But Worth Noting)

1. **No input length validation** on email, name, project name, org name, team name — arbitrarily long strings (10K+ chars) accepted. Should have reasonable limits (e.g., 255 chars for email, 500 for names).

2. **Audit endpoint has no max limit cap** — `?limit=999999` is accepted. Should enforce a maximum (e.g., 1000).

3. **Negative offset/limit in audit** are silently accepted instead of returning 400.

4. **OIDC endpoints return 500** when OIDC is not configured. Should return 503 Service Unavailable for consistency with TEE/ZK/Webhook/Replication endpoints.

5. **Project names with special chars** (slashes, spaces, dots) are accepted, which could cause confusion when used in secret paths like `/api/v1/secrets/{project}/{path}`.

6. **Adding a duplicate team member returns 500** instead of 409 Conflict.

---

## Test Coverage Summary

| Category | Endpoints Tested | Tests Run | Bugs Found | Pass Rate |
|---|---|---|---|---|
| Health/Ready | 2 | 2 | 0 | 100% |
| Auth (Register/Login/Me) | 3 | 28+ | 1 | 96% |
| Projects | 2 | 12 | 0 | 100% |
| Secrets | 5 | 24 | 0 | 100% |
| Secret Versions | 1 | 4 | 0 | 100% |
| Organizations | 3 | 12 | 0 | 100% |
| Teams | 3 | 10 | 0 | 100% |
| Team Members | 3 | 11 | 1 | 91% |
| Agents | 4 | 14 | 0 | 100% |
| Service Accounts | 2 | 11 | 0 | 100% |
| Policies | 2 | 9 | 0 | 100% |
| IAM Policies | 5 | 16 | 0 | 100% |
| Audit | 1 | 9 | 0 | 100% |
| Dashboard | 1 | 2 | 0 | 100% |
| Rotation | 4 | 8 | 0 | 100% |
| Leases | 3 | 4 | 0 | 100% |
| Webhooks | 4 | 4 | 0 | 100% |
| TEE | 3 | 4 | 0 | 100% |
| ZK Auth | 2 | 3 | 0 | 100% |
| Replication | 3 | 4 | 0 | 100% |
| OIDC | 2 | 3 | 0* | N/A |
| **TOTAL** | **~50** | **~190+** | **2** | **~99%** |

*OIDC returns 500 when not configured, which is arguably a bug (should be 503), but categorized as a warning.
