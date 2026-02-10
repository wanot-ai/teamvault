# TeamVault Security Audit Report

**Date:** 2026-02-10  
**Server:** http://localhost:8443  
**Auditor:** Automated Security Testing (OpenClaw subagent)  
**Test Users:** sec1@test.com (member), sec2@test.com (member)

---

## Summary

| # | Category | Verdict | Severity |
|---|----------|---------|----------|
| 1 | Auth Bypass | **PASS** | — |
| 2 | SQL Injection | **PASS** | — |
| 3 | Path Traversal | **PASS** | — |
| 4 | XSS Payloads | **PASS** | — |
| 5 | Token Reuse / Tampering | **PASS** | — |
| 6 | Privilege Escalation | **FAIL** | High |
| 7 | Rate Limiting | **WARN** | Medium |
| 8 | Secret Enumeration | **FAIL** | Critical |
| 9 | Large Payload | **FAIL** | Medium |
| 10 | Header Injection | **FAIL** | High |

**Overall: 4 PASS, 4 FAIL, 1 WARN** — multiple high/critical vulnerabilities found.

---

## 1. Auth Bypass — **PASS** ✅

All authentication bypass attempts were correctly rejected.

| Test | Result | HTTP Code |
|------|--------|-----------|
| No token on `/api/v1/auth/me` | `missing authorization header` | 401 |
| No token on `/api/v1/projects` | Rejected | 401 |
| No token on `/api/v1/secrets/test/key` | Rejected | 401 |
| Malformed token (`notavalidtoken`) | `invalid token` | 401 |
| Expired token (exp=1000000000) | `invalid token` | 401 |
| No "Bearer" prefix | `invalid authorization format` | 401 |
| Empty Bearer value | `invalid authorization format` | 401 |

**Analysis:** JWT validation is robust. Expired, malformed, and missing tokens are all rejected. HS256 signature verification prevents forgery.

---

## 2. SQL Injection — **PASS** ✅

All SQL injection attempts were stored as literal strings without executing. Database tables remained intact after all tests.

| Test | Input | Result |
|------|-------|--------|
| Email with SQLi | `test@test.com"; DROP TABLE users;--` | Stored as literal string (user created) |
| Login with SQLi | `" OR 1=1 --` | `invalid credentials` |
| Project name SQLi | `test"; DROP TABLE projects;--` | Stored as literal project name |
| Secret path SQLi | `key'; DROP TABLE secrets;--` | Policy denial (path stored literally) |
| UNION SELECT in path | `' UNION SELECT * FROM users--` | Policy denial (no data leaked) |
| Policy name SQLi | `test"; DROP TABLE policies;--` | `admin access required` |

**Verification:** After all injection attempts, `GET /api/v1/projects` and `GET /api/v1/auth/me` returned normal results. No tables were dropped or corrupted.

**Analysis:** The application uses parameterized queries (Go `database/sql` / `pgx`), which properly escapes all inputs. SQL injection is not possible.

---

## 3. Path Traversal — **PASS** ✅

Path traversal attempts in secret paths are handled by Go's HTTP router (`http.ServeMux`), which normalizes `..` before reaching handlers.

| Test | Input | Result |
|------|-------|--------|
| `../../etc/passwd` as secret path | `PUT /secrets/proj/../../etc/passwd` | 404 (router normalized path) |
| `../` in project name | `../../../etc` | Stored as literal project name (DB only, no filesystem access) |
| URL-encoded traversal | `%2e%2e%2fetc%2fpasswd` | Policy denied (path: `etc/passwd`) |
| GET with traversal | `GET /secrets/proj/../../etc/passwd` | 404 (router normalized) |
| Null byte injection | `test%00../../etc/passwd` | Policy denied |

**Analysis:** Secrets are stored in PostgreSQL (not filesystem), so path traversal in secret paths cannot access the filesystem. The Go HTTP router normalizes `..` segments. However, `../` is accepted as a valid project name (see WARN in input validation below).

> **WARN:** No input validation on project names — `../../../etc` is accepted as a project name. While not exploitable for filesystem access (DB-backed), it's poor hygiene and could cause issues in export/import features.

---

## 4. XSS Payloads — **PASS** ✅

The API is JSON-only and Go's `encoding/json` HTML-escapes `<`, `>`, `&` characters.

| Test | Input | Result |
|------|-------|--------|
| `<script>alert(1)</script>` as project name | Stored, returned as `\u003cscript\u003e...` | JSON-escaped |
| `<img src=x onerror=alert(1)>` in description | Stored, returned as `\u003cimg src=x onerror=alert(1)\u003e` | JSON-escaped |
| XSS in user registration name | Stored as `\u003cscript\u003e...` | JSON-escaped |
| Content-Type header | Always `application/json` | Correct |

**Analysis:** Go's `json.Marshal` automatically escapes HTML-significant characters. The API always returns `Content-Type: application/json`, preventing browser interpretation. Stored XSS in a frontend consuming this API would depend on the frontend's rendering, but the API itself is safe.

> **NOTE:** No input sanitization occurs server-side — XSS payloads are stored verbatim in the database. If a frontend renders these without escaping, stored XSS would occur. Consider adding server-side input validation.

---

## 5. Token Reuse / Tampering — **PASS** ✅

| Test | Result |
|------|--------|
| User1 token → user1 identity confirmed | `sec1@test.com` |
| User2 token → user2 identity confirmed | `sec2@test.com` |
| Tampered JWT (role changed to admin, original signature) | `invalid token` |
| Tampered JWT on admin endpoint (create policy) | `invalid token` |

**Analysis:** JWT signature verification with HS256 is working correctly. Modifying the payload invalidates the signature. Role escalation via token manipulation is not possible.

---

## 6. Privilege Escalation — **FAIL** 🔴

**Severity: HIGH**

Several admin/privileged operations are accessible to regular members.

| Test | Expected | Actual | Verdict |
|------|----------|--------|---------|
| Non-admin creating policy | 403 Forbidden | `admin access required` (403) | ✅ PASS |
| **Non-admin creating org** | **403 Forbidden** | **201 Created — org created!** | **🔴 FAIL** |
| Non-admin listing policies | Depends on design | `[]` (empty, 200) | ⚠️ WARN |
| **Non-admin viewing audit logs** | **403 Forbidden** | **200 — ALL audit events visible** | **🔴 FAIL** |
| Non-admin IAM policy creation | 403 Forbidden | Input validation error (not auth error) | ⚠️ WARN |
| **Non-admin dashboard stats** | **Filtered/restricted** | **200 — global stats visible** | **🔴 FAIL** |

### Critical Findings:

1. **Org Creation (FAIL):** Any authenticated member can create organizations. The `handleCreateOrg` handler only checks for authentication (`getUserClaims != nil`), not admin role. This should require admin privileges.

2. **Audit Log Access (FAIL):** `handleListAuditEvents` has **no authorization check** beyond authentication. Any authenticated user can view the **entire audit log**, including:
   - All other users' IDs and actions
   - All secret access patterns (who read/wrote which secrets)
   - All IP addresses of all users
   - All project names and resources
   This is a significant information disclosure vulnerability.

3. **Dashboard Stats (FAIL):** `handleDashboardStats` returns global aggregate counts (total secrets, projects, orgs, teams, active leases) to any authenticated user. This leaks organizational metadata.

4. **IAM Policy Creation:** The error is a validation error (`org_id, name, and policy_type are required`) rather than an authorization error, suggesting the handler may not have admin-only protection.

---

## 7. Rate Limiting — **WARN** ⚠️

**Severity: MEDIUM**

| Test | Result |
|------|--------|
| 250 sequential requests (curl loop) | 0 rate-limited (all 200) |
| 300 parallel requests (50 concurrent) | 88 rate-limited (212 OK) |

**Analysis:** Rate limiting works for concurrent/parallel requests but is **ineffective against sequential requests** from a single client. The token bucket is configured at 100 req/s with burst 200, which is very generous. Sequential curl requests don't exceed this rate due to TCP overhead.

**Issues:**
1. Burst of 200 is very high — allows 200 unauthenticated requests before any limiting
2. Rate limiting is bypassed via `X-Forwarded-For` header spoofing (see Test 10)
3. No separate rate limiting for authentication endpoints (login/register should have stricter limits to prevent credential brute-forcing)

---

## 8. Secret Enumeration — **FAIL** 🔴

**Severity: CRITICAL**

| Test | Expected | Actual | Verdict |
|------|----------|--------|---------|
| **User2 listing ALL projects** | **Only own projects** | **ALL projects visible (6 projects from 4 different users)** | **🔴 FAIL** |
| **User2 listing secrets in enc-test (other user's project)** | **403 or empty** | **10 secret paths enumerated (IDs, paths, creators, timestamps)** | **🔴 FAIL** |
| User2 reading secret values from other projects | 403 Forbidden | `no matching allow policy (default deny)` (403) | ✅ PASS |
| **User2 listing ALL orgs** | **Only own/joined orgs** | **ALL 3 orgs visible** | **🔴 FAIL** |
| **User2 viewing global dashboard** | **Restricted** | **Global stats revealed** | **🔴 FAIL** |

### Critical Findings:

1. **Project Enumeration:** `handleListProjects` calls `db.ListProjects()` which executes `SELECT * FROM projects` with **no user/ownership filtering**. Any authenticated user can discover all project names, descriptions, creator IDs, and timestamps.

2. **Secret Path Enumeration:** `handleListSecrets` finds the project by name (no ownership check), then returns **all secret paths** in that project. While secret **values** are protected by policy checks, the **existence and paths** of secrets are leaked. This reveals:
   - Secret naming patterns (e.g., `db/password`, `api/key`)
   - Number of secrets per project
   - Creator IDs and timestamps
   - Secret types (kv, file, json)

3. **Organization Enumeration:** `handleListOrgs` returns all organizations without ownership filtering.

4. **The policy engine only protects read/write/delete of secret VALUES** — it does not protect listing operations. This is a broken access control pattern.

---

## 9. Large Payload — **FAIL** 🔴

**Severity: MEDIUM**

| Test | Size | Result |
|------|------|--------|
| 10MB secret value | ~14MB JSON | 403 (policy denied — but body was fully read) |
| 1MB secret value | ~1.4MB JSON | 403 (policy denied — but body was fully read) |

**Code Analysis Finding:**

```go
// helpers.go - decodeJSON has NO size limit
func decodeJSON(r *http.Request, v interface{}) error {
    defer r.Body.Close()
    return json.NewDecoder(r.Body).Decode(v)
}
```

**No `http.MaxBytesReader` is used anywhere in the codebase.** The server will:
1. Accept and parse arbitrarily large JSON payloads into memory
2. Allocate unbounded memory for the decoded Go structures
3. Be vulnerable to denial-of-service via memory exhaustion

An attacker could send many concurrent multi-GB requests to crash the server with OOM.

**Recommendation:** Add `r.Body = http.MaxBytesReader(w, r.Body, maxBytes)` before JSON decoding. Suggested limit: 1MB for secrets, 64KB for other endpoints.

---

## 10. Header Injection — **FAIL** 🔴

**Severity: HIGH**

### 10a. X-Request-ID Reflection
| Test | Result |
|------|--------|
| Custom X-Request-ID header | Reflected verbatim in response: `X-Request-Id: injected-test-id` |

The server accepts and reflects client-provided X-Request-ID values without sanitization. While not directly exploitable in API responses, this could enable log injection if the request ID is logged.

### 10b. X-Forwarded-For IP Spoofing — **CRITICAL**

```go
// middleware.go
func clientIP(r *http.Request) string {
    if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
        parts := strings.SplitN(xff, ",", 2)
        return strings.TrimSpace(parts[0])
    }
    // ...
}
```

**The server unconditionally trusts `X-Forwarded-For` headers.** This enables:

1. **Rate limit bypass:** Each request with a different `X-Forwarded-For` value creates a new rate limit bucket. An attacker can send unlimited requests by cycling spoofed IPs.
2. **Audit log spoofing:** All audit events record `clientIP(r)` which uses the spoofed IP. An attacker can forge their apparent IP address in audit logs, undermining forensic analysis.
3. **IP-based access controls:** If any future features rely on client IP, they are all bypassable.

**Verified:** Requests with `X-Forwarded-For: 10.0.0.1`, `10.0.0.2`, `10.0.0.3` all succeeded independently, confirming each gets its own rate limit bucket.

### 10c. CORS Wildcard — **FAIL**

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
Access-Control-Allow-Headers: Content-Type, Authorization, X-Request-ID
```

**The server allows requests from ANY origin.** This means:
- Any malicious website can make authenticated API requests if a user has a valid token in their browser
- Combined with the JSON API, this enables full CSRF attacks from any domain
- Secret values can be exfiltrated via cross-origin requests

**Recommendation:** Restrict `Access-Control-Allow-Origin` to known frontend domains. Never use `*` for authenticated APIs.

### 10d. Missing Security Headers — **FAIL**

The following standard security headers are **completely absent**:

| Header | Status | Risk |
|--------|--------|------|
| `X-Frame-Options` | ❌ Missing | Clickjacking |
| `X-Content-Type-Options` | ❌ Missing | MIME sniffing |
| `Strict-Transport-Security` | ❌ Missing | Downgrade attacks |
| `Content-Security-Policy` | ❌ Missing | XSS mitigation |
| `X-XSS-Protection` | ❌ Missing | Legacy XSS filter |
| `Referrer-Policy` | ❌ Missing | Information leakage |

---

## Additional Observations

### Input Validation Gaps

- **Project names:** No validation — accepts `../../../etc`, `<script>alert(1)</script>`, SQL injection strings, empty-looking names
- **Secret paths:** No validation beyond URL routing — accepts any string
- **Email addresses:** No format validation — `test@test.com"; DROP TABLE users;--` is accepted
- **User names:** No validation — accepts HTML/script content

### Positive Security Features Observed

1. ✅ JWT HS256 with proper signature validation
2. ✅ Parameterized SQL queries (no SQL injection)
3. ✅ Go HTTP router normalizes `..` path segments
4. ✅ Go `json.Marshal` HTML-escapes `<>` in JSON output
5. ✅ Policy engine enforces deny-by-default for secret read/write/delete
6. ✅ Admin-only protection on policy creation endpoint
7. ✅ Audit logging with hash chain integrity
8. ✅ Secret values never logged in audit events
9. ✅ Envelope encryption for secret storage
10. ✅ Redaction middleware for error responses

---

## Recommendations (Priority Order)

### Critical (Fix Immediately)
1. **Add ownership filtering to list endpoints** — Projects, secrets listing, orgs, teams, and audit logs must filter by user/team membership
2. **Fix `X-Forwarded-For` trust** — Only trust proxy headers when behind a known reverse proxy; use `RemoteAddr` by default
3. **Add request body size limits** — Use `http.MaxBytesReader` on all endpoints

### High (Fix Soon)
4. **Restrict CORS origins** — Replace `*` with specific allowed domains
5. **Add admin-only check to org creation** — Or implement an org membership model
6. **Add admin-only check to audit log access** — Or scope to user's own events
7. **Add security headers** — `X-Frame-Options`, `X-Content-Type-Options`, `Strict-Transport-Security`, `Content-Security-Policy`

### Medium (Improve)
8. **Add per-endpoint rate limiting** — Stricter limits on auth endpoints (5/min for login, 3/min for register)
9. **Add input validation** — Reject special characters in project names, validate email format, limit string lengths
10. **Scope dashboard stats** — Return only stats relevant to the authenticated user's projects/orgs
