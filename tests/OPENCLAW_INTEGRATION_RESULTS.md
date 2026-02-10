# TeamVault × OpenClaw Integration Test Results

**Date:** 2026-02-10 09:28 KST (00:28 UTC)  
**Server:** http://localhost:8443 (Docker Compose)  
**CLI:** `bin/teamvault` (version dev, aarch64)  
**Tester:** Automated (subagent qa-openclaw-integration)

---

## Summary

| Test | Result | Notes |
|------|--------|-------|
| 1. Env hook script | ⚠️ PARTIAL | Works but has env-var bug |
| 2. JSON config validation | ✅ PASS | Valid JSON, correct API paths |
| 3. CLI run command | ✅ PASS | Single secret injection works |
| 4. Multiple secret injection | ✅ PASS | 7/7 secrets injected correctly |
| 5. Error cases | ✅ PASS | All errors handled gracefully |

**Bugs Found:** 2 (see below)

---

## Test 1: Env Hook Script (`teamvault-env-hook.sh`)

### 1a. No TEAMVAULT_URL set — ✅ PASS
```
$ (unset TEAMVAULT_URL; unset TEAMVAULT_TOKEN; bash teamvault-env-hook.sh)
[teamvault] No teamvault.json found, skipping secret injection.
Exit code: 0
```
Graceful skip when environment variables not set.

### 1b. No teamvault.json config file — ✅ PASS
```
$ TEAMVAULT_URL=http://localhost:8443 TEAMVAULT_TOKEN=fake bash teamvault-env-hook.sh
[teamvault] No teamvault.json found, skipping secret injection.
Exit code: 0
```
Graceful skip when config file doesn't exist.

### 1c. With valid config + valid credentials — ✅ PASS
```
[teamvault] Fetching secrets from TeamVault...
[teamvault] Loaded: OPENROUTER_API_KEY
[teamvault] Loaded: DISCORD_TOKEN
[teamvault] 2 secrets loaded to /tmp/tv-hook-test/.env.teamvault
[teamvault] Secrets injected into environment.
Exit code: 0
```
- Secrets fetched and injected into environment
- Temp `.env.teamvault` file cleaned up after sourcing (confirmed)

### 1d. With unreachable server — ⚠️ BUG
```
[teamvault] Fetching secrets from TeamVault...
[teamvault] Loaded: MY_KEY            ← SHOULD HAVE FAILED
[teamvault] 1 secrets loaded
```
**Bug:** The hook passes `TEAMVAULT_SERVER` and `TEAMVAULT_TOKEN` env vars to the `teamvault kv get` subprocess, but the CLI binary **ignores these env vars** and reads from `~/.teamvault/token` instead. This means the hook cannot override the server/token — it always uses the saved login credentials regardless of what `TEAMVAULT_URL`/`TEAMVAULT_TOKEN` are set to.

### 1e. Requires `teamvault` CLI in PATH — ✅ Expected
When `teamvault` binary is not in `$PATH`, the hook's Python reports:
```
[teamvault] Error fetching ...: [Errno 2] No such file or directory: 'teamvault'
```
This is expected. The SKILL.md should document the PATH requirement.

---

## Test 2: JSON Config Validation (`teamvault.json.example`)

### 2a. Valid JSON — ✅ PASS
```json
{
  "server": "https://vault.example.com:8443",
  "project": "my-openclaw-agent",
  "mappings": {
    "OPENROUTER_API_KEY": "providers/openrouter/api-key",
    "OPENAI_API_KEY": "providers/openai/api-key",
    "ANTHROPIC_API_KEY": "providers/anthropic/api-key",
    "TELEGRAM_BOT_TOKEN": "channels/telegram/token",
    "DISCORD_TOKEN": "channels/discord/token",
    "BRAVE_API_KEY": "tools/brave-search/api-key",
    "ELEVENLABS_API_KEY": "tools/elevenlabs/api-key"
  }
}
```

- **Valid JSON:** Yes
- **Required keys present:** `server`, `project`, `mappings`
- **Mapping count:** 7

### 2b. API Path Format — ✅ PASS

All secret paths follow the `category/service/key` convention and map to
`GET /api/v1/secrets/{project}/{path}`:

| Env Var | Secret Path | API Endpoint | Valid |
|---------|-------------|-------------|-------|
| `OPENROUTER_API_KEY` | `providers/openrouter/api-key` | `/api/v1/secrets/my-openclaw-agent/providers/openrouter/api-key` | ✅ |
| `OPENAI_API_KEY` | `providers/openai/api-key` | `/api/v1/secrets/my-openclaw-agent/providers/openai/api-key` | ✅ |
| `ANTHROPIC_API_KEY` | `providers/anthropic/api-key` | `/api/v1/secrets/my-openclaw-agent/providers/anthropic/api-key` | ✅ |
| `TELEGRAM_BOT_TOKEN` | `channels/telegram/token` | `/api/v1/secrets/my-openclaw-agent/channels/telegram/token` | ✅ |
| `DISCORD_TOKEN` | `channels/discord/token` | `/api/v1/secrets/my-openclaw-agent/channels/discord/token` | ✅ |
| `BRAVE_API_KEY` | `tools/brave-search/api-key` | `/api/v1/secrets/my-openclaw-agent/tools/brave-search/api-key` | ✅ |
| `ELEVENLABS_API_KEY` | `tools/elevenlabs/api-key` | `/api/v1/secrets/my-openclaw-agent/tools/elevenlabs/api-key` | ✅ |

---

## Test 3: CLI Run Command

### 3a. Single secret injection — ✅ PASS
```
$ bin/teamvault run --project openclaw-test --map "MY_VAR=providers/openrouter/api-key" -- env | grep MY_VAR
✓ Fetched 1 secret(s), launching command...
MY_VAR=sk-or-test-1234567890
```

### 3b. Value integrity — ✅ PASS
```
Stored:   sk-or-test-1234567890
Injected: sk-or-test-1234567890
MATCH: ✅
```
The injected value exactly matches the stored value.

### 3c. Secret not leaked to stderr — ✅ PASS
```
Stderr: '✓ Fetched 1 secret(s), launching command...'
```
Only the count is logged, never the secret values themselves.

### 3d. Uses `syscall.Exec` — ✅ PASS (code review)
The `run` command replaces the Go process with the child using `syscall.Exec`, which means:
- No Go runtime stays in memory with secrets
- Signal forwarding is native (OS-level)
- Exit code is preserved
- Secrets exist only in the child's environment

---

## Test 4: Multiple Secret Injection (7 secrets)

### 4a. All 7 secrets injected — ✅ PASS
```
$ bin/teamvault run --project openclaw-test \
    --map "OPENROUTER_API_KEY=providers/openrouter/api-key,OPENAI_API_KEY=providers/openai/api-key,ANTHROPIC_API_KEY=providers/anthropic/api-key,DISCORD_TOKEN=channels/discord/token,TELEGRAM_TOKEN=channels/telegram/token,BRAVE_API_KEY=tools/brave-search/api-key,ELEVENLABS_API_KEY=tools/elevenlabs/api-key" \
    -- env

✓ Fetched 7 secret(s), launching command...
OPENROUTER_API_KEY=sk-or-test-1234567890
OPENAI_API_KEY=sk-test-openai-abcdef
ANTHROPIC_API_KEY=sk-ant-test-xyz789
DISCORD_TOKEN=discord-token-abc123
TELEGRAM_TOKEN=telegram-token-def456
BRAVE_API_KEY=brave-api-key-ghi789
ELEVENLABS_API_KEY=el-api-key-jkl012
```

### 4b. Count verified — ✅ PASS
All 7 out of 7 environment variables were present and non-empty in the child process.

### 4c. All values correct — ✅ PASS
Each injected value matches the corresponding stored secret.

---

## Test 5: Error Cases

### 5a. Server unreachable — ✅ PASS (graceful failure)
```
$ bin/teamvault kv get openclaw-test/providers/openrouter/api-key
Error: failed to get secret ...: request to http://localhost:9999/... failed:
  Get "...": dial tcp 127.0.0.1:9999: connect: connection refused
Exit code: 1

$ bin/teamvault run --project openclaw-test --map "MY_VAR=..." -- echo "x"
Error: failed to fetch secret openclaw-test/... for MY_VAR: request to ... failed:
  dial tcp 127.0.0.1:9999: connect: connection refused
Exit code: 1
```
- Clear error message with connection details
- Non-zero exit code
- Child command NOT executed (correct)

### 5b. Expired/invalid token — ✅ PASS (graceful failure)
```
$ bin/teamvault kv get openclaw-test/providers/openrouter/api-key
Error: failed to get secret ...: API error (401): invalid token
Exit code: 1

$ bin/teamvault run --project openclaw-test --map "MY_VAR=..." -- echo "x"
Error: failed to fetch secret ... for MY_VAR: API error (401): invalid token
Exit code: 1
```
- Server returns 401 with `"invalid token"` message
- CLI surfaces the API error clearly
- Child command NOT executed

### 5c. Secret path doesn't exist — ✅ PASS (graceful failure)
```
$ bin/teamvault kv get openclaw-test/nonexistent/secret/path
Error: failed to get secret ...: API error (404): secret not found
Exit code: 1

$ bin/teamvault run ... --map "MY_VAR=nonexistent/secret" -- echo "x"
Error: failed to fetch secret ... for MY_VAR: API error (404): secret not found
Exit code: 1
```

### 5d. Project doesn't exist — ✅ PASS
```
$ bin/teamvault kv get no-such-project/some/secret
Error: failed to get secret ...: API error (404): project not found
Exit code: 1
```

### 5e. Invalid map format — ✅ PASS
```
$ bin/teamvault run --project openclaw-test --map "INVALID_FORMAT" -- echo "x"
Error: invalid mapping "INVALID_FORMAT": expected ENV=path format
Exit code: 1
```

### 5f. No command specified — ✅ PASS
```
$ bin/teamvault run --project openclaw-test --map "MY_VAR=providers/openrouter/api-key"
Error: no command specified. Usage: teamvault run --project PROJECT --map "ENV=path" -- CMD [ARGS...]
Exit code: 1
```

### 5g. Not logged in — ✅ PASS
```
$ bin/teamvault kv get openclaw-test/providers/openrouter/api-key
Error: not logged in. Run 'teamvault login' first
Exit code: 1
```

---

## Bugs Found

### BUG-1: CLI ignores `TEAMVAULT_SERVER`/`TEAMVAULT_TOKEN` environment variables

**Severity:** Medium  
**Location:** `cmd/teamvault/cli/config.go` → `LoadToken()`, `cmd/teamvault/cli/client.go` → `NewClient()`

**Description:** The env hook script (`teamvault-env-hook.sh`) passes `TEAMVAULT_SERVER` and `TEAMVAULT_TOKEN` environment variables to the `teamvault kv get` subprocess. However, the CLI's `NewClient()` only reads from `~/.teamvault/token` and never checks environment variables. This means:

1. The hook **always uses saved credentials** regardless of the env vars
2. If `TEAMVAULT_URL` points to server A but `~/.teamvault/token` points to server B, the hook silently uses server B
3. The hook cannot work in environments where only env vars are available (no prior `teamvault login`)

**Fix:** `NewClient()` should check `TEAMVAULT_SERVER` and `TEAMVAULT_TOKEN` env vars first, falling back to `~/.teamvault/token`. For example:
```go
func NewClient() (*APIClient, error) {
    // Check env vars first (for CI/CD, hooks, containers)
    if server := os.Getenv("TEAMVAULT_SERVER"); server != "" {
        if token := os.Getenv("TEAMVAULT_TOKEN"); token != "" {
            return &APIClient{
                BaseURL: strings.TrimRight(server, "/"),
                Token:   token,
                HTTPClient: &http.Client{Timeout: 30 * time.Second},
            }, nil
        }
    }
    // Fall back to saved token file
    tokenData, err := LoadToken()
    // ...
}
```

### BUG-2: `kv list` / `kv tree` response parsing mismatch

**Severity:** High (commands completely broken)  
**Location:** `cmd/teamvault/cli/client.go` → `ListSecrets()`

**Description:** The API's `handleListSecrets` endpoint (`internal/api/secret_handlers.go:388`) returns a **raw JSON array**:
```json
[{"id":"...","path":"...","secret_type":"kv","created_by":"...","created_at":"..."},...]
```

But the CLI's `ListSecrets()` tries to unmarshal it into:
```go
var resp struct {
    Secrets []SecretListItem `json:"secrets"`
}
```

This expects `{"secrets": [...]}`, which doesn't match the actual response format.

**Result:**
```
Error: failed to list secrets in openclaw-test: failed to parse response:
  json: cannot unmarshal array into Go value of type struct { Secrets []cli.SecretListItem ... }
```

Both `kv list` and `kv tree` are completely broken.

**Fix (either one):**

Option A — Fix the CLI to match the API:
```go
func (c *APIClient) ListSecrets(project string) ([]SecretListItem, error) {
    var items []SecretListItem
    err := c.do("GET", fmt.Sprintf("/api/v1/secrets/%s", project), nil, &items)
    if err != nil {
        return nil, err
    }
    return items, nil
}
```

Option B — Fix the API to wrap in an object (breaking change):
```go
writeJSON(w, http.StatusOK, map[string]interface{}{"secrets": items})
```

**Recommended:** Option A (fix CLI), since the API is already deployed and other consumers may depend on the array format. Also note the API's `SecretListItem` has `secret_type` field but the CLI struct expects `version` — the struct fields don't match either.

---

## Additional Notes

### Field mismatch in `SecretListItem`

The CLI's `SecretListItem` struct:
```go
type SecretListItem struct {
    ID          string `json:"id"`
    Path        string `json:"path"`
    Description string `json:"description"`
    Version     int    `json:"version"`
    CreatedAt   string `json:"created_at"`
}
```

The API actually returns:
```json
{
  "id": "...",
  "path": "...",
  "secret_type": "kv",
  "created_by": "...",
  "created_at": "..."
}
```

Missing from CLI: `secret_type`, `created_by`, `metadata`  
Missing from API: `version`, `description` (for list endpoint)

### Security Observations (Positive)

- ✅ Secret values never appear in stderr/logs
- ✅ `syscall.Exec` ensures no Go runtime persists with secrets in memory
- ✅ Temp env file created with 0600 permissions and deleted immediately after sourcing
- ✅ Auth errors return proper 401 status codes without leaking info
- ✅ Non-existent paths return 404 (no path enumeration)
- ✅ Invalid map formats caught before any API calls

### Integration Architecture

```
┌─────────────────────────────────────────────┐
│  OpenClaw Gateway                           │
│                                             │
│  Pre-start hook: teamvault-env-hook.sh      │
│    ├─ Reads teamvault.json (mappings)       │
│    ├─ Calls teamvault CLI for each secret   │
│    ├─ Writes temp .env file (0600)          │
│    ├─ Sources into environment              │
│    └─ Deletes temp file                     │
│                                             │
│  OR direct CLI usage:                       │
│    teamvault run --map "VAR=path" -- cmd    │
│    ├─ Fetches all secrets via API           │
│    ├─ Builds env with secrets               │
│    └─ syscall.Exec replaces process         │
└─────────────────────────────────────────────┘
        │
        ▼ HTTPS + Bearer token
┌─────────────────────────────────────────────┐
│  TeamVault Server (:8443)                   │
│  GET /api/v1/secrets/{project}/{path}       │
│  Policy engine enforces access control      │
└─────────────────────────────────────────────┘
```
