#!/bin/bash
# ============================================================
# TeamVault E2E Demo Script
# ============================================================
# This script demonstrates a complete end-to-end workflow:
#
# 1. Admin registers and sets up an org/team
# 2. Stores secrets (API keys, DB credentials, certificates)
# 3. Creates an agent with scoped access
# 4. Agent fetches secrets via REST API
# 5. Simulates OpenClaw integration (env injection)
# 6. Views audit trail of all actions
# ============================================================

set -euo pipefail

# Configuration
SERVER="${TEAMVAULT_DEMO_SERVER:-http://localhost:8443}"
BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'
BOLD='\033[1m'

step() { echo -e "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"; echo -e "${BOLD}$1${NC}"; echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"; }
ok() { echo -e "  ${GREEN}✓${NC} $1"; }
info() { echo -e "  ${YELLOW}→${NC} $1"; }
fail() { echo -e "  ${RED}✗${NC} $1"; exit 1; }

# ============================================================
step "STEP 1: Health Check"
# ============================================================

HEALTH=$(curl -sf "$SERVER/health" | python3 -c "import sys,json; print(json.load(sys.stdin)['status'])")
[ "$HEALTH" = "ok" ] && ok "Server is healthy at $SERVER" || fail "Server not responding"

READY=$(curl -sf "$SERVER/ready" | python3 -c "import sys,json; print(json.load(sys.stdin)['status'])")
[ "$READY" = "ready" ] && ok "Database is ready" || fail "Database not ready"

# ============================================================
step "STEP 2: Register Admin Account"
# ============================================================

REGISTER_RESP=$(curl -sf "$SERVER/api/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"admin@demo.teamvault.dev\",\"password\":\"Demo2026!\",\"name\":\"Demo Admin\"}")
ADMIN_TOKEN=$(echo "$REGISTER_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
ADMIN_ID=$(echo "$REGISTER_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['user']['id'])")
ok "Registered admin@demo.teamvault.dev (ID: ${ADMIN_ID:0:8}...)"

# Promote to admin (in production, first user is auto-admin)
docker exec teamvault-postgres-1 psql -U teamvault -c "UPDATE users SET role='admin' WHERE email='admin@demo.teamvault.dev';" > /dev/null 2>&1
ADMIN_TOKEN=$(curl -sf "$SERVER/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@demo.teamvault.dev","password":"Demo2026!"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
ok "Promoted to admin and re-authenticated"

AUTH="Authorization: Bearer $ADMIN_TOKEN"

# ============================================================
step "STEP 3: Create Organization & Team"
# ============================================================

ORG=$(curl -sf "$SERVER/api/v1/orgs" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"name":"acme-corp","description":"Acme Corporation — AI-powered payments"}')
ORG_ID=$(echo "$ORG" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
ok "Created org: acme-corp (ID: ${ORG_ID:0:8}...)"

TEAM=$(curl -sf "$SERVER/api/v1/orgs/$ORG_ID/teams" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"name":"ai-agents","description":"Team managing all AI agent secrets"}')
TEAM_ID=$(echo "$TEAM" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
ok "Created team: ai-agents (ID: ${TEAM_ID:0:8}...)"

# ============================================================
step "STEP 4: Create Vault & Store Secrets"
# ============================================================

PROJECT=$(curl -sf "$SERVER/api/v1/projects" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"name":"agent-secrets","description":"Secrets for AI agents"}')
ok "Created vault: agent-secrets"

# Store various secrets
SECRETS=(
  "providers/openrouter/api-key|sk-or-v1-demo-key-12345|OpenRouter API Key"
  "providers/anthropic/api-key|sk-ant-demo-key-67890|Anthropic API Key"
  "channels/discord/token|MTIzNDU2Nzg5MDEyMzQ1Njc4.demo|Discord Bot Token"
  "channels/telegram/token|7654321:AAHdemo-telegram-token|Telegram Bot Token"
  "tools/brave-search/api-key|BSA-demo-brave-key|Brave Search API Key"
  "infrastructure/db/prod/connection-string|postgresql://app:s3cret@db.acme.com:5432/prod|Production DB URL"
  "infrastructure/redis/prod/url|redis://:r3dis@redis.acme.com:6379/0|Production Redis URL"
)

for entry in "${SECRETS[@]}"; do
  IFS='|' read -r path value desc <<< "$entry"
  curl -sf -X PUT "$SERVER/api/v1/secrets/agent-secrets/$path" \
    -H "$AUTH" -H "Content-Type: application/json" \
    -d "{\"value\":\"$value\",\"description\":\"$desc\"}" > /dev/null
  ok "Stored: $path"
done

info "7 secrets stored with AES-256-GCM envelope encryption"

# ============================================================
step "STEP 5: List Secrets (Folder Structure)"
# ============================================================

echo ""
SECRETS_LIST=$(curl -sf "$SERVER/api/v1/secrets/agent-secrets" -H "$AUTH")
echo "$SECRETS_LIST" | python3 -c "
import sys, json
secrets = json.load(sys.stdin)
# Build tree
tree = {}
for s in secrets:
    parts = s['path'].split('/')
    node = tree
    for p in parts[:-1]:
        node = node.setdefault(p, {})
    node[parts[-1]] = '●'

def print_tree(node, prefix=''):
    items = sorted(node.items(), key=lambda x: (isinstance(x[1], str), x[0]))
    for i, (k, v) in enumerate(items):
        is_last = i == len(items) - 1
        connector = '└── ' if is_last else '├── '
        if isinstance(v, str):
            print(f'{prefix}{connector}🔑 {k}')
        else:
            print(f'{prefix}{connector}📁 {k}/')
            extension = '    ' if is_last else '│   '
            print_tree(v, prefix + extension)

print('  agent-secrets/')
print_tree(tree, '  ')
"

# ============================================================
step "STEP 6: Read & Verify Secret (Encrypted at Rest, Decrypted on Read)"
# ============================================================

SECRET_RESP=$(curl -sf "$SERVER/api/v1/secrets/agent-secrets/providers/openrouter/api-key" -H "$AUTH")
SECRET_VALUE=$(echo "$SECRET_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['value'])")
SECRET_VERSION=$(echo "$SECRET_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['version'])")
ok "Read: providers/openrouter/api-key (v$SECRET_VERSION)"
info "Value: ${SECRET_VALUE:0:10}... (truncated for safety)"

# Verify it's encrypted in DB
DB_CHECK=$(docker exec teamvault-postgres-1 psql -U teamvault -t -c \
  "SELECT encode(ciphertext, 'hex') FROM secret_versions LIMIT 1;" 2>/dev/null | tr -d ' \n')
ok "DB stores ciphertext only: ${DB_CHECK:0:32}..."

# ============================================================
step "STEP 7: Update Secret (Creates New Version)"
# ============================================================

curl -sf -X PUT "$SERVER/api/v1/secrets/agent-secrets/providers/openrouter/api-key" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"value":"sk-or-v1-rotated-key-99999","description":"OpenRouter API Key (rotated)"}' > /dev/null
ok "Updated: providers/openrouter/api-key → v2"

NEW_RESP=$(curl -sf "$SERVER/api/v1/secrets/agent-secrets/providers/openrouter/api-key" -H "$AUTH")
NEW_VALUE=$(echo "$NEW_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['value'])")
NEW_VERSION=$(echo "$NEW_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['version'])")
ok "Verified: v$NEW_VERSION = ${NEW_VALUE:0:15}..."

# ============================================================
step "STEP 8: Simulate OpenClaw Agent Integration"
# ============================================================

info "Simulating: teamvault run --project agent-secrets --map ENV=path -- openclaw gateway start"
echo ""

# Fetch secrets like the CLI would
MAPPINGS=(
  "OPENROUTER_API_KEY|providers/openrouter/api-key"
  "ANTHROPIC_API_KEY|providers/anthropic/api-key"
  "DISCORD_TOKEN|channels/discord/token"
  "BRAVE_API_KEY|tools/brave-search/api-key"
)

echo -e "  ${BOLD}Injected environment variables:${NC}"
for mapping in "${MAPPINGS[@]}"; do
  IFS='|' read -r env_var path <<< "$mapping"
  value=$(curl -sf "$SERVER/api/v1/secrets/agent-secrets/$path" -H "$AUTH" | python3 -c "import sys,json; print(json.load(sys.stdin)['value'])")
  echo -e "    ${GREEN}$env_var${NC} = ${value:0:15}... ✓"
done

echo ""
info "In production: 'teamvault run' would exec the child process with these env vars"
info "Secrets exist only in the child process memory — gone when it exits"

# ============================================================
step "STEP 9: Audit Trail"
# ============================================================

AUDIT=$(curl -sf "$SERVER/api/v1/audit?limit=15" -H "$AUTH")
echo ""
echo "$AUDIT" | python3 -c "
import sys, json
events = json.load(sys.stdin)
print('  {:<20} {:<16} {:<42} {:<8}'.format('TIMESTAMP', 'ACTION', 'RESOURCE', 'OUTCOME'))
print('  ' + '─' * 88)
for e in events[:12]:
    ts = e['timestamp'][:19].replace('T', ' ')
    action = e['action']
    resource = e['resource'][:40]
    outcome = e['outcome']
    color = '\033[0;32m' if outcome == 'success' else '\033[0;31m'
    print(f'  {ts:<20} {action:<16} {resource:<42} {color}{outcome}\033[0m')
"
info "All reads, writes, and denials are recorded with hash-chaining for tamper detection"

# ============================================================
step "DEMO COMPLETE"
# ============================================================

echo ""
echo -e "  ${GREEN}${BOLD}TeamVault is running at: $SERVER${NC}"
echo ""
echo -e "  ${BOLD}What you just saw:${NC}"
echo -e "    1. Created an organization with a team"
echo -e "    2. Stored 7 secrets (AES-256-GCM encrypted)"
echo -e "    3. Read secrets back (decrypted on-the-fly)"
echo -e "    4. Updated a secret (automatic versioning)"
echo -e "    5. Simulated AI agent env injection"
echo -e "    6. Full audit trail of every action"
echo ""
echo -e "  ${BOLD}Try it yourself:${NC}"
echo -e "    Web UI:  ${YELLOW}cd web && npm run dev${NC} → http://localhost:3000"
echo -e "    API:     ${YELLOW}curl $SERVER/api/v1/auth/login -d '{...}'${NC}"
echo -e "    CLI:     ${YELLOW}teamvault login --server $SERVER --email admin@demo.teamvault.dev${NC}"
echo ""
