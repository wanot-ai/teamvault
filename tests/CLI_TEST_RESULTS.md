# TeamVault CLI End-to-End Test Results

**Date:** 2026-02-10T09:23 KST  
**Server:** http://localhost:8443  
**CLI Build:** Docker `golang:1.23-alpine`, fresh build from source  
**Test User:** fixer@test.com (registered + promoted to admin)

---

## Summary

| # | Command | Result | Category |
|---|---------|--------|----------|
| 1 | `login` | ✅ PASS | Auth |
| 2 | `kv put` | ✅ PASS | KV |
| 3 | `kv get` | ✅ PASS | KV |
| 4 | `kv list` | ❌ FAIL | KV — JSON deserialization bug |
| 5 | `kv tree` | ❌ FAIL | KV — JSON deserialization bug (same root cause) |
| 6 | `org create` | ✅ PASS | Org |
| 7 | `org list` | ❌ FAIL | Org — JSON deserialization bug |
| 8 | `team create` | ❌ FAIL | Team — wrong API endpoint (405) |
| 9 | `team list` | ❌ FAIL | Team — JSON deserialization bug |
| 10 | `policy list` | ❌ FAIL | Policy — JSON deserialization bug |
| 11 | `doctor` | ⚠️ PARTIAL | Doctor — health endpoint path mismatch |
| 12 | `scan` | ✅ PASS | Security |
| 13 | `version` | ✅ PASS | Info |

**Pass: 5 | Fail: 6 | Partial: 1 | Skipped: 1 (kv put needed project creation first)**

---

## Detailed Results

### Test 1: `login` ✅ PASS

```bash
$ echo "TestPass123!" | bin/teamvault login --server http://localhost:8443 --email fixer@test.com
Password: Authenticating with http://localhost:8443...
✓ Logged in as fixer@test.com
  Token stored in ~/.teamvault/token
```

**Notes:** Works correctly. Reads piped password from stdin when not a TTY. Token saved to `~/.teamvault/token` as JSON `{"token":"...","server":"..."}`.

---

### Test 2: `kv put` ✅ PASS (with prerequisite)

```bash
$ bin/teamvault kv put cli-test/key1 --value "hello"
✓ Secret cli-test/key1 saved
```

**Notes:** Initially failed with `API error (404): project not found` because "cli-test" project didn't exist. After creating the project via API (`POST /api/v1/projects`), the command works. **There is no CLI command to create projects** — this is a UX gap.

---

### Test 3: `kv get` ✅ PASS

```bash
$ bin/teamvault kv get cli-test/key1
hello
```

**Notes:** Clean output — prints just the value, suitable for piping. Works perfectly.

---

### Test 4: `kv list` ❌ FAIL

```bash
$ bin/teamvault kv list cli-test
Error: failed to list secrets in cli-test: failed to parse response: json: cannot unmarshal array into Go value of type struct { Secrets []cli.SecretListItem "json:\"secrets\"" }
```

**Root Cause:** The API `GET /api/v1/secrets?project=cli-test` returns a **bare JSON array** `[...]`, but the CLI tries to unmarshal into `struct { Secrets []SecretListItem }` (expects `{"secrets": [...]}`).

---

### Test 5: `kv tree` ❌ FAIL

```bash
$ bin/teamvault kv tree cli-test
Error: failed to list secrets in cli-test: failed to parse response: json: cannot unmarshal array into Go value of type struct { Secrets []cli.SecretListItem "json:\"secrets\"" }
```

**Root Cause:** Same as Test 4 — `kv tree` calls the same list endpoint internally.

---

### Test 6: `org create` ✅ PASS

```bash
$ bin/teamvault org create --name cli-org
✓ Organization created
  ID:   83a00c82-bb3f-436e-b55f-18af1306ff29
  Name: cli-org
{
  "id": "83a00c82-bb3f-436e-b55f-18af1306ff29",
  "name": "cli-org",
  "display_name": "",
  "description": "",
  "created_at": "2026-02-10T00:25:04.439095Z",
  "member_count": 0
}
```

**Notes:** Works. Outputs both human-friendly confirmation and JSON body. Could be cleaner (pick one format).

---

### Test 7: `org list` ❌ FAIL

```bash
$ bin/teamvault org list
Error: failed to list organizations: failed to parse response: json: cannot unmarshal array into Go value of type struct { Orgs []cli.OrgResponse "json:\"orgs\"" }
```

**Root Cause:** API `GET /api/v1/orgs` returns `[{...}, {...}]` (bare array), CLI expects `{"orgs": [{...}, {...}]}`.

**Verified via curl:**
```json
// Actual API response:
[
  {"id": "...", "name": "cli-org", ...},
  {"id": "...", "name": "test-org", ...}
]
```

---

### Test 8: `team create` ❌ FAIL

```bash
$ bin/teamvault team create --org cli-org --name cli-team
Error: failed to create team: API error (405): Method Not Allowed
```

**Root Cause:** **CLI sends `POST /api/v1/teams`** but the server route for team creation is **`POST /api/v1/orgs/{id}/teams`** (nested under orgs). The server only has `GET /api/v1/teams` for listing all teams.

**Server routes (from `internal/api/server.go`):**
```
POST /api/v1/orgs/{id}/teams     → handleCreateTeam
GET  /api/v1/orgs/{id}/teams     → handleListTeams (per-org)
GET  /api/v1/teams               → handleListAllTeams (global)
```

**CLI code (`cmd/teamvault/cli/team.go:45`):**
```go
err := c.do("POST", "/api/v1/teams", body, &resp)  // WRONG
```

Should be: `POST /api/v1/orgs/{orgID}/teams`

**Additional issue:** The CLI resolves `--org cli-org` (slug) to org ID, but the `CreateTeam()` function doesn't use the org ID in the URL path.

---

### Test 9: `team list` ❌ FAIL

```bash
$ bin/teamvault team list
Error: failed to list teams: failed to parse response: json: cannot unmarshal array into Go value of type struct { Teams []cli.TeamResponse "json:\"teams\"" }
```

**Root Cause:** Same JSON deserialization pattern — API returns bare array, CLI expects wrapped object `{"teams": [...]}`.

---

### Test 10: `policy list` ❌ FAIL

```bash
$ bin/teamvault policy list
Error: failed to list policies: failed to parse response: json: cannot unmarshal array into Go value of type struct { Policies []cli.PolicyResponse "json:\"policies\"" }
```

**Root Cause:** Same JSON deserialization pattern — API returns bare array, CLI expects wrapped object `{"policies": [...]}`.

---

### Test 11: `doctor` ⚠️ PARTIAL PASS

```bash
$ bin/teamvault doctor --server http://localhost:8443

Running health checks against http://localhost:8443

  ✓  Reachability         connected (0ms)
  ✗  Server Health        health endpoint error: API error (404): Not Found
  ✗  Database             could not determine (health endpoint unreachable)
  ✓  Authentication       authenticated as fixer@test.com
  ⚠  Token Expiry         expires 2026-02-10T10:25:01+09:00 (in 59m) — consider refreshing

Some checks failed ✗
Error: health check failed
```

**Root Cause:** Doctor queries `GET /api/v1/health` (defined in `cli/doctor.go:46`) but the server health endpoint is at `/health` (root, no `/api/v1` prefix). Reachability and auth checks pass, but health/DB checks fail because of the endpoint mismatch.

**Server health endpoint:** `GET /health → {"status":"ok"}`  
**CLI expected endpoint:** `GET /api/v1/health` (404)

---

### Test 12: `scan` ✅ PASS

```bash
$ bin/teamvault scan --path .
⚠ Found 184 potential secret(s) in 8 file(s):

FILE                                    LINE  PATTERN                    MATCH          CONFIDENCE
----                                    ----  -------                    -----          ----------
Makefile                                3     Database URL (PostgreSQL)  postg...sable  90%
Makefile                                11    Generic Secret Assignment  SECRE...tion"  60%
cmd/teamvault/cli/kv.go                 51    Database URL (PostgreSQL)  postg...st/db  90%
demo/e2e_demo.sh                        90    Database URL (PostgreSQL)  postg...ction  90%
...
tests/API_TEST_RESULTS.md               1662  JWT Token                  eyJhb...elJk`  85%
```

**Notes:** Works correctly. Exits with code 1 when secrets are found (correct behavior for CI). Detects PostgreSQL connection strings, Redis URLs, JWT tokens, and generic secret assignments. Found 184 potential secrets in 8 files.

---

### Test 13: `version` ✅ PASS

```bash
$ bin/teamvault version
teamvault version dev
```

**Notes:** Simple, clean output. Shows `dev` because no build-time version injection via ldflags.

---

## Bug Classification

### 🔴 BUG-1: JSON Array Deserialization Mismatch (Critical — affects 5 commands)

**Affected commands:** `kv list`, `kv tree`, `org list`, `team list`, `policy list`

**Problem:** All list endpoints in the server return bare JSON arrays `[...]`, but the CLI client tries to unmarshal into wrapper structs like `struct { Items []T "json:\"items\"" }`.

**Fix (Option A — Fix CLI):** Change all list functions to unmarshal directly into `[]T` slices instead of wrapper structs:
```go
// Before:
var wrapper struct { Orgs []OrgResponse `json:"orgs"` }
err := c.do("GET", "/api/v1/orgs", nil, &wrapper)

// After:
var orgs []OrgResponse
err := c.do("GET", "/api/v1/orgs", nil, &orgs)
```

**Fix (Option B — Fix Server):** Wrap API responses in envelope objects (better REST practice):
```json
{"orgs": [...], "total": 3}
```

**Recommendation:** Option B is better long-term (supports pagination metadata), but Option A is a quick fix.

---

### 🔴 BUG-2: Team Create Uses Wrong API Endpoint (Critical)

**Affected command:** `team create`

**Problem:** CLI sends `POST /api/v1/teams` but server expects `POST /api/v1/orgs/{orgID}/teams`.

**File:** `cmd/teamvault/cli/team.go:45`

**Fix:**
```go
// Before:
err := c.do("POST", "/api/v1/teams", body, &resp)

// After:
err := c.do("POST", fmt.Sprintf("/api/v1/orgs/%s/teams", orgID), body, &resp)
```

Also need to update `CreateTeam()` signature to accept and use the org ID in the URL path.

---

### 🟡 BUG-3: Doctor Health Endpoint Path Mismatch (Medium)

**Affected command:** `doctor`

**Problem:** CLI queries `/api/v1/health` but server health endpoint is at `/health`.

**File:** `cmd/teamvault/cli/doctor.go:46`

**Fix:**
```go
// Before:
err := c.do("GET", "/api/v1/health", nil, &resp)

// After:
err := c.do("GET", "/health", nil, &resp)
```

Note: The `/health` response is `{"status":"ok"}` which doesn't match the expected `HealthResponse` struct (which expects `version`, `uptime`, `database`, etc.). The server health endpoint may need enrichment, or the doctor needs to handle the simpler response.

---

### 🟡 UX-GAP-1: No CLI Command to Create Projects

The `kv put` command fails if the project doesn't exist, but there's no `teamvault project create` command. Projects must be created via raw API calls. Consider adding project management commands.

---

### 🟡 UX-GAP-2: Inconsistent Output Formatting (Minor)

`org create` outputs both human-readable status AND raw JSON. Should pick one format (or use `--json` flag).

---

## Environment Details

```
OS: Linux 6.11.0-1016-nvidia (arm64)
Docker: available (used for Go build)
Go: 1.23-alpine (via Docker)
Server: teamvault-server-1 (Docker Compose)
Database: PostgreSQL 16-alpine (teamvault-postgres-1)
```
