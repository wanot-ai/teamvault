# TeamVault Edge Case Test Results

**Date:** 2026-02-10  
**Server:** http://localhost:8443  
**Tester:** Automated QA Script  
**Admin User:** admin@test.com (ID: 6566c231-9cde-4375-9c62-5f9c6390aabb)

---

## Setup

Before running secret-related tests, an admin policy was created to allow the test user full access:
```
POST /api/v1/policies
{
    "name": "admin-all-secrets",
    "effect": "allow",
    "actions": ["read","write","delete","list"],
    "resource_pattern": "*/*",
    "subject_type": "user",
    "subject_id": "6566c231-9cde-4375-9c62-5f9c6390aabb"
}
→ 201 Created
```

---

## Test 1: Register with email longer than 255 chars

**Request:** `POST /api/v1/auth/register` with 256-character email (`aaa...aaa@b.com`)

**Expected:** 400-level error rejecting overly long email (RFC 5321 limit is 254 chars)

**Actual Response:**
- Status Code: `201`
- Body: Registration succeeded. User created with 256-char email, JWT returned.

**Verdict:** ❌ FAIL — No email length validation. Server accepts emails exceeding RFC limits. Should reject with 400.

---

## Test 2: Register with password shorter than 4 chars

**Request:** `POST /api/v1/auth/register` with `password="ab"` (2 chars)

**Expected:** 400-level error rejecting short password

**Actual Response:**
- Status Code: `400`
- Body: `{"error":"password must be at least 8 characters"}`

**Verdict:** ✅ PASS — Correctly rejects short passwords. Minimum is 8 chars (stricter than the 4-char test threshold).

---

## Test 3: Login with empty body

**Request:** `POST /api/v1/auth/login` with empty body

**Expected:** 400-level error

**Actual Response:**
- Status Code: `400`
- Body: `{"error":"invalid request body"}`

**Verdict:** ✅ PASS — Properly rejects empty request body.

---

## Test 4: Login with JSON syntax error

**Request:** `POST /api/v1/auth/login` with body `{bad json`

**Expected:** 400-level error about invalid JSON

**Actual Response:**
- Status Code: `400`
- Body: `{"error":"invalid request body"}`

**Verdict:** ✅ PASS — Properly rejects malformed JSON.

---

## Test 5: Create project with special chars in name

### 5a: `test/project`

**Request:** `POST /api/v1/projects` with `name="test/project"`

**Expected:** 400 (rejected due to slash) or 201 (accepted)

**Actual Response:**
- Status Code: `201`
- Body: Project created with name `test/project`

**Verdict:** ⚠️ PASS (functional) — Server accepts slashes in project names. This could cause path confusion with secret paths like `{project}/{path}`. Consider validating project names to disallow `/`.

### 5b: `test project`

**Request:** `POST /api/v1/projects` with `name="test project"`

**Expected:** 400 or 201

**Actual Response:**
- Status Code: `201`
- Body: Project created with name `test project`

**Verdict:** ✅ PASS — Spaces accepted. Reasonable behavior.

### 5c: `test@project`

**Request:** `POST /api/v1/projects` with `name="test@project"`

**Expected:** 400 or 201

**Actual Response:**
- Status Code: `201`
- Body: Project created with name `test@project`

**Verdict:** ✅ PASS — Special chars accepted.

---

## Test 6: Create project with name longer than 200 chars

**Request:** `POST /api/v1/projects` with 201-character name (`aaa...aaa`)

**Expected:** 400-level error rejecting overly long name

**Actual Response:**
- Status Code: `201`
- Body: Project created with 201-char name.

**Verdict:** ❌ FAIL — No project name length validation. Should reject names over a reasonable limit (e.g., 128 or 200 chars).

---

## Test 7: Put secret with path containing `..`

**Request:** `PUT /api/v1/secrets/testproject/../etc/passwd` with `value="evil"`

**Expected:** 400-level error rejecting path traversal

**Actual Response (standard):**
- Status Code: `404`
- Body: `{"error":"project not found"}`
- Note: HTTP client resolves `testproject/../etc` to `etc`, so it looks for project `etc` (not found).

**Request (URL-encoded):** `PUT /api/v1/secrets/testproject/%2e%2e/etc/passwd`

**Actual Response:**
- Status Code: `200`
- Body: Secret created with path `../etc/passwd` stored in `testproject`

**Verdict:** ❌ FAIL — **SECURITY ISSUE**: URL-encoded `..` (`%2e%2e`) bypasses path resolution and stores `../etc/passwd` as a literal path. While this may not cause filesystem traversal (secrets are in DB), it's a validation gap. The server should reject paths containing `..` segments.

---

## Test 8: Put secret with empty value

**Request:** `PUT /api/v1/secrets/testproject/empty-test` with `value=""`

**Expected:** 400 (rejected) or 201 (accepted empty string)

**Actual Response:**
- Status Code: `400`
- Body: `{"error":"value is required"}`

**Verdict:** ✅ PASS — Correctly rejects empty secret values.

---

## Test 9: Put secret with value larger than 1MB

**Request:** `PUT /api/v1/secrets/testproject/bigvalue` with 1,048,577-byte value

**Expected:** 400/413 error rejecting oversized value

**Actual Response:**
- Status Code: `200`
- Body: Secret created successfully with 1MB+ value.

**Verdict:** ❌ FAIL — No secret value size limit enforced. A 1MB+ secret value was accepted. This could lead to resource exhaustion. Should enforce a maximum size (e.g., 64KB or 256KB for secrets).

---

## Test 10: Put secret with unicode value (emoji, CJK characters)

**Request:** `PUT /api/v1/secrets/testproject/unicode-test` with `value="🔑こんにちは世界🌍 émojis and ñ"`

**Expected:** 201 success and value preserved on roundtrip

**Actual Response (PUT):**
- Status Code: `200`
- Body: Secret created successfully.

**Actual Response (GET):**
- Status Code: `200`
- Body: `{"value":"🔑こんにちは世界🌍 émojis and ñ"}` — Unicode perfectly preserved.

**Verdict:** ✅ PASS — Unicode values (emoji, CJK, accented chars) are correctly stored and retrieved through encryption/decryption cycle.

---

## Test 11: Get secret that was soft-deleted

**Request:** Create secret at `testproject/delete-me2`, then `DELETE`, then `GET`

**PUT Response:** `200` — Secret created  
**DELETE Response:** `200` — `{"status":"deleted"}`  
**GET Response:**
- Status Code: `404`
- Body: `{"error":"secret not found"}`

**Verdict:** ✅ PASS — Soft-deleted secrets correctly return 404 on subsequent reads.

---

## Test 12: Create org with unicode name

**Request:** `POST /api/v1/orgs` with `name="テスト組織🏢"`

**Expected:** 201 (accepted) or 404 (endpoint not in MVP)

**Actual Response:**
- Status Code: `201`
- Body: `{"id":"096267f5-...","name":"テスト組織🏢",...}`

**Verdict:** ✅ PASS — Unicode org names are accepted and preserved.

---

## Test 13: Create team with empty name

**Request:** `POST /api/v1/orgs/{orgId}/teams` with `name=""`

**Expected:** 400-level error

**Actual Response:**
- Status Code: `400`
- Body: `{"error":"name is required"}`

**Verdict:** ✅ PASS — Correctly rejects empty team names.

---

## Test 14: Add non-existent user to team (random UUID)

**Request:** `POST /api/v1/teams/{teamId}/members` with `user_id="00000000-0000-0000-0000-000000000099"`

**Expected:** 400 or 404 error

**Actual Response:**
- Status Code: `500`
- Body: `{"error":"failed to add team member"}`

**Verdict:** ❌ FAIL — Returns 500 Internal Server Error instead of a proper 400/404. This is likely an unhandled foreign key constraint violation in PostgreSQL. Should catch the DB error and return 404 ("user not found").

---

## Test 15: Create policy with malformed JSON in policy_doc

**Request:** `POST /api/v1/policies` with `conditions="this is not json"` (string, not JSON object)

**Expected:** 400 (rejected) or 201 (string stored as-is)

**Actual Response:**
- Status Code: `201`
- Body: Policy created with `conditions:"this is not json"`

**Verdict:** ⚠️ PASS (functional) — The conditions field is stored as a raw string without JSON validation. While this doesn't crash, invalid conditions may cause runtime policy evaluation errors. Consider validating that `conditions` is valid JSON or a structured object.

---

## Test 16: Query audit with invalid date format

**Request:** `GET /api/v1/audit?from=not-a-date`

**Expected:** 400 (bad request) or 200 (parameter ignored)

**Actual Response:**
- Status Code: `200`
- Body: Full audit log returned (parameter silently ignored).

**Verdict:** ✅ PASS — Invalid date format is gracefully handled by ignoring the filter. Could improve by returning 400 for obviously invalid date formats.

---

## Test 17: Query audit with limit=0

**Request:** `GET /api/v1/audit?limit=0`

**Expected:** 200 with empty results or 400

**Actual Response:**
- Status Code: `200`
- Body: Full audit log returned (limit=0 treated as "use default").

**Verdict:** ✅ PASS — Handled gracefully. Server falls back to default limit.

---

## Test 18: Query audit with limit=-1

**Request:** `GET /api/v1/audit?limit=-1`

**Expected:** 400 (rejected) or 200 (clamped to default)

**Actual Response:**
- Status Code: `200`
- Body: Full audit log returned.

**Verdict:** ✅ PASS — Negative limit handled gracefully (falls back to default).

---

## Test 19: Query audit with limit=10000

**Request:** `GET /api/v1/audit?limit=10000`

**Expected:** 200 (clamped to max) or 400

**Actual Response:**
- Status Code: `200`
- Body: Full audit log returned.

**Verdict:** ✅ PASS — Large limit handled. Note: unclear if it's actually clamped to a maximum or if 10000 is accepted as-is. With more audit entries, this could cause performance issues.

---

## Test 20: Send request with Content-Type: text/plain

**Request:** `POST /api/v1/auth/login` with `Content-Type: text/plain` and JSON body

**Expected:** 400 (unsupported media type) or 200 (parsed anyway)

**Actual Response:**
- Status Code: `200`
- Body: Login succeeded with JWT token returned.

**Verdict:** ✅ PASS — Server parses the body as JSON regardless of Content-Type. Lenient but functional. Strict API would return 415 Unsupported Media Type.

---

## Test 21: Send GET to POST-only endpoint

**Request:** `GET /api/v1/auth/register`

**Expected:** 405 Method Not Allowed

**Actual Response:**
- Status Code: `405`
- Body: `Method Not Allowed`

**Verdict:** ✅ PASS — Correctly rejects wrong HTTP method.

---

## Test 22: Send POST to GET-only endpoint

**Request:** `POST /api/v1/audit`

**Expected:** 405 Method Not Allowed

**Actual Response:**
- Status Code: `405`
- Body: `Method Not Allowed`

**Verdict:** ✅ PASS — Correctly rejects wrong HTTP method.

---

## Test 23: Double-slash in URL path

**Request:** `GET //api/v1//projects`

**Expected:** 200 (path normalized) or 404

**Actual Response:**
- Status Code: `301`
- Body: `<a href="/api/v1/projects">Moved Permanently</a>.`

**Verdict:** ✅ PASS — Server normalizes double slashes and redirects to the canonical path. Good behavior from Go's default HTTP mux.

---

## Test 24: Trailing slash in URL path

**Request:** `GET /api/v1/projects/`

**Expected:** 200 (slash ignored) or 404

**Actual Response:**
- Status Code: `404`
- Body: `404 page not found`

**Verdict:** ⚠️ PASS (strict routing) — Trailing slash causes 404 due to strict route matching. This is Go's standard ServeMux behavior. Could add a redirect from `/projects/` to `/projects` for better developer experience.

---

## Test 25: Request with very large Authorization header (10KB)

**Request:** `GET /api/v1/projects` with 10,240-byte Authorization header

**Expected:** 401/413/431 error

**Actual Response:**
- Status Code: `401`
- Body: `{"error":"invalid token"}`

**Verdict:** ✅ PASS — Server processes the large header without crashing and correctly identifies it as an invalid token.

---

## Test 26: Multiple concurrent logins for same user

**Request:** 5x `POST /api/v1/auth/login` in rapid sequence for the same user

**Expected:** All return 200 with valid tokens

**Actual Response:**
- All 5 requests returned `200` with unique JWT tokens.

**Verdict:** ✅ PASS — Multiple concurrent sessions are supported. Each login generates a fresh JWT.

---

## Test 27: Update a secret 100 times rapidly (version stress)

**Request:** 100x `PUT /api/v1/secrets/testproject/version-stress2` with incrementing values

**Expected:** All 100 updates succeed, 101 versions created (v0 + 100 updates)

**Actual Response:**
- Errors: 0/100 (all succeeded)
- Version count: 101 (confirmed via `/versions` endpoint)

**Verdict:** ✅ PASS — Version system handles rapid sequential updates correctly. No race conditions or version conflicts.

---

## Test 28: Create 100 projects

**Request:** 100x `POST /api/v1/projects` with names `bulk-project-1` through `bulk-project-100`

**Expected:** All 100 creations succeed

**Actual Response:**
- Errors: 0/100 (all succeeded)

**Verdict:** ✅ PASS — System handles bulk project creation without issues.

---

## Test 29: List projects when there are 100+

**Request:** `GET /api/v1/projects` (after creating 100+ projects)

**Expected:** 200 with all projects listed

**Actual Response:**
- Status Code: `200`
- Body: 115 projects returned in the response.

**Verdict:** ✅ PASS — All projects returned. Note: no pagination observed. With very large numbers of projects, this could become a performance issue. Consider implementing pagination.

---

## Test 30: Store secret with newlines and tabs in value

**Request:** `PUT /api/v1/secrets/testproject/whitespace-test` with `value="line1\nline2\ttab\nline3"`

**Expected:** 201 and value roundtrips correctly

**PUT Response:** `200` — Secret created  
**GET Response:** `200` — `value:"line1\nline2\ttab\nline3"` (whitespace preserved)

**Verdict:** ✅ PASS — Newlines and tabs in secret values are correctly preserved through the encryption/decryption cycle.

---

# Summary

| # | Test | Status |
|---|------|--------|
| 1 | Register with email > 255 chars | ❌ FAIL |
| 2 | Register with password < 4 chars | ✅ PASS |
| 3 | Login with empty body | ✅ PASS |
| 4 | Login with JSON syntax error | ✅ PASS |
| 5 | Create project with special chars (`/`, space, `@`) | ✅ PASS ⚠️ |
| 6 | Create project with name > 200 chars | ❌ FAIL |
| 7 | Put secret with path containing `..` | ❌ FAIL |
| 8 | Put secret with empty value | ✅ PASS |
| 9 | Put secret with value > 1MB | ❌ FAIL |
| 10 | Put secret with unicode value | ✅ PASS |
| 11 | Get soft-deleted secret | ✅ PASS |
| 12 | Create org with unicode name | ✅ PASS |
| 13 | Create team with empty name | ✅ PASS |
| 14 | Add non-existent user to team | ❌ FAIL |
| 15 | Create policy with malformed conditions | ✅ PASS ⚠️ |
| 16 | Query audit with invalid date | ✅ PASS |
| 17 | Query audit with limit=0 | ✅ PASS |
| 18 | Query audit with limit=-1 | ✅ PASS |
| 19 | Query audit with limit=10000 | ✅ PASS |
| 20 | Content-Type: text/plain | ✅ PASS |
| 21 | GET to POST-only endpoint | ✅ PASS |
| 22 | POST to GET-only endpoint | ✅ PASS |
| 23 | Double-slash in URL | ✅ PASS |
| 24 | Trailing slash in URL | ✅ PASS ⚠️ |
| 25 | Very large Authorization header | ✅ PASS |
| 26 | Multiple concurrent logins | ✅ PASS |
| 27 | 100 rapid secret updates | ✅ PASS |
| 28 | Create 100 projects | ✅ PASS |
| 29 | List 100+ projects | ✅ PASS |
| 30 | Secret with newlines and tabs | ✅ PASS |

## Totals

| Metric | Count |
|--------|-------|
| **Total Tests** | 30 |
| **✅ PASS** | 25 |
| **❌ FAIL** | 5 |
| **⚠️ Warnings** | 3 |

---

# Issues Found (Prioritized)

### 🔴 Critical

1. **Path Traversal via URL-encoded `..` (Test 7)** — `%2e%2e` in secret paths is accepted and stored literally as `../etc/passwd`. While secrets are DB-stored (not filesystem), this is a validation gap that could confuse path-based policy matching.

### 🟡 Medium

2. **No email length validation (Test 1)** — Emails exceeding 255 chars (RFC 5321 limit of 254) are accepted. Add max length validation.

3. **No project name length validation (Test 6)** — Arbitrarily long project names accepted. Add a reasonable max (128-200 chars).

4. **No secret value size limit (Test 9)** — 1MB+ values accepted without restriction. Could enable resource exhaustion attacks. Add a max size (e.g., 64KB-256KB).

5. **500 error when adding non-existent user to team (Test 14)** — Unhandled foreign key constraint violation returns 500 instead of 404. Catch the DB error and return a proper client error.

### 🟢 Low / Informational

6. **Project names allow `/` (Test 5a)** — Slashes in project names may conflict with `{project}/{path}` URL routing.

7. **Policy conditions not validated as JSON (Test 15)** — Arbitrary strings stored in `conditions` field. May cause runtime evaluation errors.

8. **Trailing slash returns 404 (Test 24)** — Strict routing; consider adding redirects for better DX.

9. **Audit limit=0 and limit=-1 not rejected (Tests 17-18)** — Silently falls back to default. Consider explicit validation.

10. **No pagination on project list (Test 29)** — 115+ projects returned in a single response. Add pagination support for scalability.
