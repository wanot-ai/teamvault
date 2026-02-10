# TeamVault Web UI Issues (Playwright MCP Test)

## Tested Pages & Results

| Page | URL | Status | Errors |
|:---|:---|:---:|:---|
| Dashboard | `/dashboard` | ERROR | `GET /api/v1/dashboard/stats` → 404 |
| Organizations | `/orgs` | OK | No errors |
| Leases | `/leases` | OK | No errors |
| IAM Policies | `/policies` | ERROR | `GET /api/v1/iam/policies` → 404 (URL mismatch: frontend calls `/iam/policies`, server has `/iam-policies`) |
| Audit Log | `/audit` | OK | No errors |
| Settings | `/settings` | OK | No errors |

## Issues Found

### Issue 1: `GET /api/v1/dashboard/stats` → 404
- **Page:** Dashboard (`/dashboard`)
- **Console:** `[ERROR] Failed to load resource: the server responded with a status of 404`
- **Root Cause:** Endpoint not implemented in backend
- **Fix:** Add `handleDashboardStats` handler returning counts from DB

### Issue 2: `GET /api/v1/iam/policies` → 404 (URL Mismatch)
- **Page:** IAM Policies (`/policies`)
- **Console:** `[ERROR] ApiError: API Error 404: Not Found`
- **UI shows:** "Failed to load IAM policies"
- **Root Cause:** Frontend `api.ts` calls `/iam/policies` but server registers `/iam-policies`
- **Fix:** Either change frontend to `/iam-policies` OR add server alias `/iam/policies`

### Issue 3 (from earlier API test): `GET /api/v1/teams` → 404
- **Root Cause:** No global teams list endpoint (only per-org: `/orgs/{id}/teams`)
- **Fix:** Add `GET /api/v1/teams` for listing all teams

### Issue 4 (from earlier API test): `GET /rotation-status` → 405
- **Root Cause:** Rotation endpoints only accept POST, but Web UI uses GET for status check
- **Fix:** Add GET handler or separate status endpoint

## CORS
- FIXED: OPTIONS preflight returns 204 with `Access-Control-Allow-Origin: *`
