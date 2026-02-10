# TeamVault Web UI — Playwright QA Test Results

**Date:** 2026-02-10 09:22–09:28 KST  
**Tester:** Automated (Playwright MCP via mcporter, chromium headless shell)  
**Target:** http://142.231.83.48:3000 (Web) / http://142.231.83.48:8443 (API)  
**Login:** sigrid@wanot.ai / teamvault2026  

---

## Summary

| # | Test | Result | Severity |
|---|------|--------|----------|
| 1 | Login & Dashboard | ✅ PASS | — |
| 2 | Create Project (UI) | ❌ FAIL | **Critical** |
| 3 | Create Secrets (UI) | ❌ FAIL | **Critical** |
| 4 | Verify secrets listed | ❌ FAIL | **Critical** |
| 5 | Secret detail page | ❌ BLOCKED | **Critical** |
| 6 | Organizations — create org (UI) | ❌ FAIL | **Critical** |
| 7 | Audit Log page | ✅ PASS | — |
| 8 | IAM Policies page | ✅ PASS | — |
| 9 | Leases page | ⚠️ PARTIAL | Medium |
| 10 | Settings page | ✅ PASS | — |

**Overall: 4 PASS, 4 FAIL, 1 BLOCKED, 1 PARTIAL**

---

## Detailed Results

### Test 1: Login & Navigate to Dashboard
- **Action:** Navigated to http://142.231.83.48:3000
- **Result:** ✅ **PASS** — Auto-redirected to `/dashboard`. Logged in as "Sigrid Jin (sigrid@wanot.ai)".
- **Observations:**
  - Dashboard loads correctly with stats cards (Total Secrets, Total Projects, Active Leases, Recent Rotations)
  - Quick Actions section present (Create Project, Store Secret, Manage Leases)
  - Recent Activity widget shows last 10 audit events
  - Projects list shows all existing projects with names, timestamps, descriptions
  - Navigation sidebar has all expected links: Projects, Organizations, Leases, IAM Policies, Audit Log, Settings
- **Console errors:** None

---

### Test 2: Create Project "qa-test-project" (via UI)
- **Action:** Clicked "New Project" button → Dialog appeared → Filled Name: "qa-test-project", Description: "QA test project for Playwright testing" → Clicked "Create"
- **Result:** ❌ **FAIL**
- **Error displayed:** Toast notification: **"Failed to create project"**
- **Console errors:**
  ```
  [ERROR] Failed to load resource: the server responded with a status of 500 (Internal Server Error) @ http://142.231.83.48:8443/api/v1/projects
  [ERROR] ApiError: API Error 500: Internal Server Error at s (chunks/296a02b9b037c755.js:1:839)
  ```
- **Root cause analysis:** The API endpoint `POST /api/v1/projects` returned HTTP 500 when called from the UI. However, the **same request succeeds via curl** (HTTP 201 Created). This suggests the UI may be sending a malformed request body, incorrect auth header, or the API has a race condition.
- **Workaround:** Project was successfully created via direct API call with curl.

---

### Test 3: Create Secrets (db/url, db/password, api/key) via UI
- **Action:** Navigated to project page → Clicked "New Secret" → Filled Path: "db/url", Value: "postgresql://localhost:5432/teamvault", Description: "Database connection URL" → Clicked "Create Secret"
- **Result:** ❌ **FAIL**
- **Error displayed:** Toast notification: **"Failed to create secret"**
- **Console errors:**
  ```
  [ERROR] Failed to load resource: the server responded with a status of 404 (Not Found)
      @ http://142.231.83.48:8443/api/v1/secrets/a15f23a9-e749-47c7-94d5-8f30154405b4/db%2Furl
  [ERROR] ApiError: API Error 404: Not Found
  ```
- **Root cause (BUG):** The UI sends the secret creation request to:
  ```
  PUT /api/v1/secrets/{PROJECT_UUID}/{PATH}
  ```
  But the API expects:
  ```
  PUT /api/v1/secrets/{PROJECT_NAME}/{PATH}
  ```
  The UI uses the project **UUID** (e.g., `a15f23a9-e749-47c7-94d5-8f30154405b4`) where the API expects the project **name** (e.g., `qa-test-project`). This causes a 404 because no project with that UUID-as-name exists.
- **Additional bug:** The forward slash in the path (`db/url`) is URL-encoded as `db%2Furl` in the UI request, which may also cause issues depending on API routing.
- **Workaround:** Secrets were successfully created via direct API call:
  ```
  curl -X PUT http://142.231.83.48:8443/api/v1/secrets/qa-test-project/db/url
  ```

---

### Test 4: Verify Secrets Listed on Project Page
- **Action:** Navigated to project detail page (`/projects/{uuid}`)
- **Result:** ❌ **FAIL** — Always shows "No secrets yet" with "Failed to load secrets" error
- **Error displayed:** Toast notification: **"Failed to load secrets"**
- **Console errors:**
  ```
  [ERROR] Failed to load resource: the server responded with a status of 404 (Not Found)
      @ http://142.231.83.48:8443/api/v1/secrets/a15f23a9-e749-47c7-94d5-8f30154405b4
  [ERROR] ApiError: API Error 404: Not Found
  ```
- **Root cause (same as Test 3):** The UI calls `GET /api/v1/secrets/{PROJECT_UUID}` but the API expects `GET /api/v1/secrets/{PROJECT_NAME}`. This is the **same UUID-vs-name mismatch bug** affecting both listing and creation of secrets.
- **Impact:** No secrets are ever visible on any project detail page in the UI, even though they exist in the API.

---

### Test 5: Click on Secret — Detail Page
- **Action:** Could not test — secrets never appear in the UI due to the bug in Test 4.
- **Result:** ❌ **BLOCKED** by Test 4 failure

---

### Test 6: Organizations — Create "qa-org"
- **Action:** Navigated to `/orgs` → Clicked "New Organization" → Filled Name: "qa-org", Slug: auto-generated "qa-org", Description: "QA testing organization" → Clicked "Create Organization"
- **Result:** ❌ **FAIL**
- **Error displayed:** Toast notification: **"Failed to create organization"**
- **Console errors:**
  ```
  [ERROR] Failed to load resource: the server responded with a status of 500 (Internal Server Error)
      @ http://142.231.83.48:8443/api/v1/orgs
  [ERROR] ApiError: API Error 500: Internal Server Error
  ```
- **Notes:** The same request via curl returns 201 Created successfully. The Organizations page itself renders correctly with org cards showing name, member count, and team count.

---

### Test 7: Audit Log Page
- **Action:** Navigated to `/audit` via sidebar link
- **Result:** ✅ **PASS**
- **Observations:**
  - Page heading: "Audit Log" with subtitle "Track all actions across your vault"
  - Filter panel present with: Action (dropdown), Outcome (dropdown), Actor ID (text input), From Date, To Date
  - Table with columns: Timestamp, Action, Actor, Resource, Outcome, IP
  - Events displayed with proper formatting (e.g., "Feb 10, 09:27:04", "auth.register", Success badge)
  - "Showing 1 event" count displayed at bottom
- **Console errors:** None on this page

---

### Test 8: IAM Policies Page
- **Action:** Navigated to `/policies` via sidebar link
- **Result:** ✅ **PASS**
- **Observations:**
  - Page heading: "IAM Policies" with subtitle "Manage access control policies for your organization"
  - "New Policy" button present
  - Three tabs: **RBAC**, **ABAC**, **PBAC** — all functional
  - RBAC tab: Shows empty state "No RBAC policies" with "New RBAC Policy" button
  - ABAC tab: Shows empty state "No ABAC policies" with "New ABAC Policy" button  
  - PBAC tab: Shows empty state "No PBAC policies" with "New PBAC Policy" button
  - Tab switching works correctly
- **Console errors:** None on this page

---

### Test 9: Leases Page
- **Action:** Navigated to `/leases` via sidebar link
- **Result:** ⚠️ **PARTIAL** — Page structure loads but data fetch fails intermittently
- **Observations:**
  - Page heading: "Leases" with subtitle "Manage dynamic secret leases · Auto-refreshes every 30s"
  - Buttons present: "Refresh", "Issue Lease"
  - Empty state shows: "No leases" / "Issue a lease to provide time-limited access to dynamic secrets"
- **Error displayed:** Toast: **"Failed to load leases"**
- **Console errors:**
  ```
  [ERROR] Failed to load resource: net::ERR_CONNECTION_REFUSED @ http://142.231.83.48:8443/api/v1/leases
  [ERROR] TypeError: Failed to fetch
  [ERROR] Failed to load resource: net::ERR_CONNECTION_REFUSED @ http://142.231.83.48:8443/api/v1/projects
  ```
- **Notes:** The API server was intermittently crashing/restarting during tests, causing ERR_CONNECTION_REFUSED errors. The Leases UI structure itself is correct. On earlier page loads (before API crashes), the dashboard showed "5 Active Leases", confirming the feature works when API is available.

---

### Test 10: Settings Page
- **Action:** Navigated to `/settings` via sidebar link
- **Result:** ✅ **PASS**
- **Observations:**
  - Page heading: "Settings" with subtitle "Manage service accounts and access policies"
  - Two tabs: **Service Accounts** (default), **Policies**
  - **Service Accounts tab:**
    - "New Service Account" button present
    - Table with columns: Name, Project, Scopes, Created, Expires
    - 4 existing service accounts displayed correctly (qa-sa-auth, qa-sa-default, qa-sa-ttl, qa-sa)
    - Scopes displayed as badges (read, write)
    - Expiry displayed as "Never" or date
  - **Policies tab:**
    - "New Policy" button present
    - Table with columns: Name, Effect, Actions, Resource, Subject, Created
    - 2 existing policies displayed correctly (qa-deny: deny/delete, qa-policy: allow/read+write)
    - Effect and action badges rendered properly (allow=green-ish, deny=red-ish)
- **Console errors:** None on this page

---

## Critical Bugs Found

### BUG-1: Project creation via UI fails with 500 Internal Server Error
- **Severity:** Critical
- **Page:** Dashboard (`/dashboard`)
- **Action:** Click "New Project" → Fill form → Click "Create"
- **Expected:** Project is created and appears in list
- **Actual:** Toast "Failed to create project", API returns 500
- **API endpoint:** `POST /api/v1/projects`
- **Notes:** Same request succeeds via curl. May be related to how the frontend sends the auth token or request body.

### BUG-2: Secrets UI uses project UUID instead of project name in API calls
- **Severity:** Critical (breaks all secret operations)
- **Page:** Project detail (`/projects/{uuid}`)
- **Affected operations:** List secrets, Create secret
- **Expected URL pattern:** `GET/PUT /api/v1/secrets/{project_name}/{path}`
- **Actual URL pattern:** `GET/PUT /api/v1/secrets/{project_uuid}/{path}` 
- **Example:**
  - UI sends: `GET /api/v1/secrets/a15f23a9-e749-47c7-94d5-8f30154405b4`
  - Should send: `GET /api/v1/secrets/qa-test-project`
- **Impact:** No secrets are ever visible in any project page. Secret creation always fails.

### BUG-3: Organization creation via UI fails with 500 Internal Server Error
- **Severity:** Critical
- **Page:** Organizations (`/orgs`)
- **Action:** Click "New Organization" → Fill form → Click "Create Organization"
- **Expected:** Organization is created and appears in list
- **Actual:** Toast "Failed to create organization", API returns 500
- **Notes:** Same as BUG-1 pattern — curl works, UI doesn't.

### BUG-4: API server unstable — frequent crashes/restarts
- **Severity:** High
- **Observed:** API at port 8443 becomes unreachable (`ERR_CONNECTION_REFUSED`) multiple times during testing
- **Impact:** All data is lost on restart (in-memory storage), causing cascading failures across all UI pages
- **Frequency:** Crashed at least 2 times during ~6 minutes of testing

### BUG-5: Project detail heading shows "Project" instead of project name after API restart
- **Severity:** Medium
- **Page:** Project detail (`/projects/{uuid}`)
- **Observed:** After API restart, navigating to project detail shows heading "Project" instead of the actual project name. The project metadata fetch appears to fail silently, falling back to generic heading.

---

## Additional Observations

1. **Auto-registration:** The web UI appears to auto-register/auto-login users. Navigating to the root URL immediately redirects to `/dashboard` with a logged-in session. No explicit login form was encountered.

2. **First-user role inconsistency:** The first user registered on a fresh API server sometimes gets "admin" role and sometimes "member" role. This may be a race condition in the bootstrap process.

3. **UI structure quality:** The overall UI structure is well-designed with proper navigation, empty states, table layouts, filter panels, tab controls, and toast notifications. The issues are primarily in API integration.

4. **No error recovery:** When API calls fail, the UI shows a toast notification but doesn't offer a retry mechanism. The user must manually reload the page.

5. **URL encoding of secret paths:** The forward slash in secret paths (e.g., `db/url`) is URL-encoded to `db%2Furl` in API requests. This may cause routing issues on the backend.

---

## Test Environment Notes

- **Browser:** Chromium headless shell (`--no-sandbox`)
- **Tool:** Playwright MCP via mcporter v0.7.3
- **OS:** Linux 6.11.0-1016-nvidia (arm64)
- **Node.js:** v22.22.0
- **API server stability:** Very poor — crashed multiple times during testing, data is not persisted
