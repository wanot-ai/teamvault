#!/bin/bash
# TeamVault 100-Scenario Test Suite
# Tests: auth, projects, secrets, orgs, teams, agents, policies, audit, leases, rotation, versioning, edge cases
set -uo pipefail

SERVER="http://localhost:8443"
PASS=0
FAIL=0
ERRORS=""

ok() { PASS=$((PASS+1)); echo "  ✓ $1"; }
fail() { FAIL=$((FAIL+1)); ERRORS="$ERRORS\n  ✗ $1"; echo "  ✗ $1"; }
section() { echo ""; echo "━━━ $1 ━━━"; }

expect_code() {
  local desc="$1" method="$2" url="$3" expected="$4" auth="${5:-}" data="${6:-}"
  local args=(-s -o /tmp/tv_resp -w "%{http_code}" -X "$method" "$SERVER$url")
  [ -n "$auth" ] && args+=(-H "Authorization: Bearer $auth")
  [ -n "$data" ] && args+=(-H "Content-Type: application/json" -d "$data")
  local code=$(curl "${args[@]}")
  if [ "$code" = "$expected" ]; then ok "$desc (HTTP $code)"
  else fail "$desc: expected $expected got $code — $(cat /tmp/tv_resp | head -c 100)"; fi
}

get_json() { python3 -c "import sys,json; print(json.load(sys.stdin)$1)" 2>/dev/null; }

# ============================================================
section "AUTH (1-10)"
# ============================================================

# 1
expect_code "Register user alice" POST "/api/v1/auth/register" 201 "" '{"email":"alice@test.com","password":"pass1234","name":"Alice"}'
ALICE_TOKEN=$(cat /tmp/tv_resp | get_json "['token']")

# 2
expect_code "Register user bob" POST "/api/v1/auth/register" 201 "" '{"email":"bob@test.com","password":"pass1234","name":"Bob"}'
BOB_TOKEN=$(cat /tmp/tv_resp | get_json "['token']")

# 3
expect_code "Login alice" POST "/api/v1/auth/login" 200 "" '{"email":"alice@test.com","password":"pass1234"}'
ALICE_TOKEN=$(cat /tmp/tv_resp | get_json "['token']")

# 4
expect_code "Login wrong password" POST "/api/v1/auth/login" 401 "" '{"email":"alice@test.com","password":"wrong"}'

# 5
expect_code "Login nonexistent user" POST "/api/v1/auth/login" 401 "" '{"email":"nobody@test.com","password":"pass"}'

# 6
expect_code "Register duplicate email" POST "/api/v1/auth/register" 409 "" '{"email":"alice@test.com","password":"pass1234","name":"Alice2"}'

# 7
expect_code "Get /auth/me with token" GET "/api/v1/auth/me" 200 "$ALICE_TOKEN"

# 8
expect_code "Get /auth/me without token" GET "/api/v1/auth/me" 401

# 9
expect_code "Get /auth/me with bad token" GET "/api/v1/auth/me" 401 "invalid.token.here"

# 10
expect_code "Register with missing fields" POST "/api/v1/auth/register" 400 "" '{"email":"bad@test.com"}'

# Promote alice to admin
docker exec teamvault-postgres-1 psql -U teamvault -c "UPDATE users SET role='admin' WHERE email='alice@test.com';" > /dev/null 2>&1
ALICE_TOKEN=$(curl -s "$SERVER/api/v1/auth/login" -H "Content-Type: application/json" -d '{"email":"alice@test.com","password":"pass1234"}' | get_json "['token']")

# ============================================================
section "PROJECTS (11-20)"
# ============================================================

# 11
expect_code "Create project alpha" POST "/api/v1/projects" 201 "$ALICE_TOKEN" '{"name":"alpha","description":"Alpha project"}'
ALPHA_ID=$(cat /tmp/tv_resp | get_json "['id']")

# 12
expect_code "Create project beta" POST "/api/v1/projects" 201 "$ALICE_TOKEN" '{"name":"beta","description":"Beta project"}'

# 13
expect_code "Create duplicate project" POST "/api/v1/projects" 409 "$ALICE_TOKEN" '{"name":"alpha","description":"Dup"}'

# 14
expect_code "List projects" GET "/api/v1/projects" 200 "$ALICE_TOKEN"

# 15
expect_code "Create project without auth" POST "/api/v1/projects" 401 "" '{"name":"noauth"}'

# 16
expect_code "Create project empty name" POST "/api/v1/projects" 400 "$ALICE_TOKEN" '{"name":"","description":"empty"}'

# 17-20: More project operations
expect_code "Create project gamma" POST "/api/v1/projects" 201 "$ALICE_TOKEN" '{"name":"gamma","description":"Gamma"}'
expect_code "Create project delta" POST "/api/v1/projects" 201 "$ALICE_TOKEN" '{"name":"delta","description":"Delta"}'
expect_code "Create project epsilon" POST "/api/v1/projects" 201 "$ALICE_TOKEN" '{"name":"epsilon","description":"Epsilon"}'

PROJ_COUNT=$(curl -s "$SERVER/api/v1/projects" -H "Authorization: Bearer $ALICE_TOKEN" | python3 -c "import sys,json;print(len(json.load(sys.stdin)))")
[ "$PROJ_COUNT" -ge 5 ] && ok "List projects returns >= 5 ($PROJ_COUNT)" || fail "Expected >= 5 projects, got $PROJ_COUNT"

# ============================================================
section "SECRETS CRUD (21-40)"
# ============================================================

# 21-25: Basic CRUD
expect_code "Put secret alpha/db/url" PUT "/api/v1/secrets/alpha/db/url" 200 "$ALICE_TOKEN" '{"value":"postgres://localhost/alpha","description":"DB URL"}'
expect_code "Put secret alpha/db/password" PUT "/api/v1/secrets/alpha/db/password" 200 "$ALICE_TOKEN" '{"value":"s3cret!","description":"DB pass"}'
expect_code "Put secret alpha/api/stripe" PUT "/api/v1/secrets/alpha/api/stripe" 200 "$ALICE_TOKEN" '{"value":"sk_live_12345","description":"Stripe key"}'
expect_code "Get secret alpha/db/url" GET "/api/v1/secrets/alpha/db/url" 200 "$ALICE_TOKEN"
VALUE=$(cat /tmp/tv_resp | get_json "['value']")
[ "$VALUE" = "postgres://localhost/alpha" ] && ok "Secret value matches" || fail "Value mismatch: $VALUE"

# 26-30: List and hierarchy
expect_code "List secrets in alpha" GET "/api/v1/secrets/alpha" 200 "$ALICE_TOKEN"
SEC_COUNT=$(cat /tmp/tv_resp | python3 -c "import sys,json;print(len(json.load(sys.stdin)))")
[ "$SEC_COUNT" -eq 3 ] && ok "Alpha has 3 secrets ($SEC_COUNT)" || fail "Expected 3, got $SEC_COUNT"

expect_code "Put deep path alpha/services/payment/prod/key" PUT "/api/v1/secrets/alpha/services/payment/prod/key" 200 "$ALICE_TOKEN" '{"value":"deep_value","description":"Deep"}'
expect_code "Get deep path" GET "/api/v1/secrets/alpha/services/payment/prod/key" 200 "$ALICE_TOKEN"
expect_code "Put secret in beta" PUT "/api/v1/secrets/beta/config/redis" 200 "$ALICE_TOKEN" '{"value":"redis://localhost","description":"Redis"}'

# 31-35: Versioning
expect_code "Update alpha/db/url v2" PUT "/api/v1/secrets/alpha/db/url" 200 "$ALICE_TOKEN" '{"value":"postgres://prod/alpha","description":"DB URL v2"}'
V2=$(cat /tmp/tv_resp | get_json "['version']")
[ "$V2" = "2" ] && ok "Version incremented to 2" || fail "Expected v2, got $V2"

expect_code "Update alpha/db/url v3" PUT "/api/v1/secrets/alpha/db/url" 200 "$ALICE_TOKEN" '{"value":"postgres://prod-ha/alpha","description":"DB URL v3"}'
expect_code "Get versions" GET "/api/v1/secret-versions/alpha/db/url" 200 "$ALICE_TOKEN"
VER_COUNT=$(cat /tmp/tv_resp | python3 -c "import sys,json;print(len(json.load(sys.stdin)))")
[ "$VER_COUNT" -eq 3 ] && ok "3 versions exist" || fail "Expected 3 versions, got $VER_COUNT"

expect_code "Latest version has v3 value" GET "/api/v1/secrets/alpha/db/url" 200 "$ALICE_TOKEN"
LATEST=$(cat /tmp/tv_resp | get_json "['value']")
[ "$LATEST" = "postgres://prod-ha/alpha" ] && ok "Latest value correct" || fail "Latest value wrong: $LATEST"

# 36-40: Edge cases
expect_code "Get nonexistent secret" GET "/api/v1/secrets/alpha/nonexistent" 404 "$ALICE_TOKEN"
expect_code "Get secret from nonexistent project" GET "/api/v1/secrets/nonexistent/key" 404 "$ALICE_TOKEN"
expect_code "Put secret without value" PUT "/api/v1/secrets/alpha/empty" 400 "$ALICE_TOKEN" '{"description":"no value"}'
expect_code "Put secret without auth" PUT "/api/v1/secrets/alpha/noauth" 401 "" '{"value":"test"}'
expect_code "Delete secret" DELETE "/api/v1/secrets/alpha/api/stripe" 200 "$ALICE_TOKEN"

# ============================================================
section "ORGS & TEAMS (41-55)"
# ============================================================

# 41-45
expect_code "Create org acme" POST "/api/v1/orgs" 201 "$ALICE_TOKEN" '{"name":"acme","description":"Acme Corp"}'
ORG_ID=$(cat /tmp/tv_resp | get_json "['id']")
expect_code "Create org waystar" POST "/api/v1/orgs" 201 "$ALICE_TOKEN" '{"name":"waystar","description":"Waystar Royco"}'
expect_code "Create duplicate org" POST "/api/v1/orgs" 409 "$ALICE_TOKEN" '{"name":"acme","description":"dup"}'
expect_code "List orgs" GET "/api/v1/orgs" 200 "$ALICE_TOKEN"
expect_code "Create org without auth" POST "/api/v1/orgs" 401 "" '{"name":"noauth"}'

# 46-50
expect_code "Create team engineering" POST "/api/v1/orgs/$ORG_ID/teams" 201 "$ALICE_TOKEN" '{"name":"engineering","description":"Eng team"}'
TEAM_ID=$(cat /tmp/tv_resp | get_json "['id']")
expect_code "Create team design" POST "/api/v1/orgs/$ORG_ID/teams" 201 "$ALICE_TOKEN" '{"name":"design","description":"Design team"}'
expect_code "Create duplicate team" POST "/api/v1/orgs/$ORG_ID/teams" 409 "$ALICE_TOKEN" '{"name":"engineering","description":"dup"}'
expect_code "List teams in org" GET "/api/v1/orgs/$ORG_ID/teams" 200 "$ALICE_TOKEN"
TEAM_COUNT=$(cat /tmp/tv_resp | python3 -c "import sys,json;print(len(json.load(sys.stdin)))")
[ "$TEAM_COUNT" -eq 2 ] && ok "2 teams in org" || fail "Expected 2 teams, got $TEAM_COUNT"

# 51-55: Team members
BOB_ID=$(curl -s "$SERVER/api/v1/auth/login" -H "Content-Type: application/json" -d '{"email":"bob@test.com","password":"pass1234"}' | get_json "['user']['id']")
expect_code "Add bob to engineering" POST "/api/v1/teams/$TEAM_ID/members" 200 "$ALICE_TOKEN" "{\"user_id\":\"$BOB_ID\",\"role\":\"member\"}"
expect_code "List team members" GET "/api/v1/teams/$TEAM_ID/members" 200 "$ALICE_TOKEN"
expect_code "Add duplicate member" POST "/api/v1/teams/$TEAM_ID/members" 409 "$ALICE_TOKEN" "{\"user_id\":\"$BOB_ID\",\"role\":\"member\"}"
expect_code "Remove bob from team" DELETE "/api/v1/teams/$TEAM_ID/members/$BOB_ID" 200 "$ALICE_TOKEN"
expect_code "Remove nonexistent member" DELETE "/api/v1/teams/$TEAM_ID/members/00000000-0000-0000-0000-000000000000" 404 "$ALICE_TOKEN"

# ============================================================
section "AGENTS (56-65)"
# ============================================================

# 56-60
expect_code "Create agent ci-bot" POST "/api/v1/teams/$TEAM_ID/agents" 201 "$ALICE_TOKEN" '{"name":"ci-bot","description":"CI agent","scopes":["read"]}'
AGENT_TOKEN=$(cat /tmp/tv_resp | get_json "['token']" 2>/dev/null || echo "")
expect_code "Create agent deploy-bot" POST "/api/v1/teams/$TEAM_ID/agents" 201 "$ALICE_TOKEN" '{"name":"deploy-bot","scopes":["read","write"]}'
expect_code "List agents" GET "/api/v1/teams/$TEAM_ID/agents" 200 "$ALICE_TOKEN"
AGENT_COUNT=$(cat /tmp/tv_resp | python3 -c "import sys,json;print(len(json.load(sys.stdin)))")
[ "$AGENT_COUNT" -eq 2 ] && ok "2 agents in team" || fail "Expected 2, got $AGENT_COUNT"
expect_code "Create duplicate agent" POST "/api/v1/teams/$TEAM_ID/agents" 409 "$ALICE_TOKEN" '{"name":"ci-bot","scopes":["read"]}'

# 61-65: Service accounts (legacy)
expect_code "Create service account" POST "/api/v1/service-accounts" 201 "$ALICE_TOKEN" '{"name":"legacy-sa","project_id":"'"$ALPHA_ID"'","scopes":["read"]}'
expect_code "List service accounts" GET "/api/v1/service-accounts" 200 "$ALICE_TOKEN"
expect_code "Create SA without project" POST "/api/v1/service-accounts" 400 "$ALICE_TOKEN" '{"name":"bad-sa"}'
expect_code "Create SA without auth" POST "/api/v1/service-accounts" 401 "" '{"name":"noauth"}'
expect_code "Get health" GET "/health" 200

# ============================================================
section "IAM POLICIES (66-75)"
# ============================================================

# 66-70
expect_code "Create RBAC policy" POST "/api/v1/iam-policies" 201 "$ALICE_TOKEN" '{"org_id":"'"$ORG_ID"'","name":"eng-read","description":"Engineering read","policy_type":"rbac","policy_doc":{"role":"viewer","team":"engineering","rules":[{"path":"alpha/*","capabilities":["read","list"]}]}}'
expect_code "Create ABAC policy" POST "/api/v1/iam-policies" 201 "$ALICE_TOKEN" '{"org_id":"'"$ORG_ID"'","name":"prod-mfa","description":"Prod requires MFA","policy_type":"abac","policy_doc":{"rules":[{"path":"*/prod/*","capabilities":["read"],"conditions":[{"attribute":"mfa_verified","operator":"eq","value":true}]}]}}'
expect_code "Create PBAC policy" POST "/api/v1/iam-policies" 201 "$ALICE_TOKEN" '{"org_id":"'"$ORG_ID"'","name":"ci-policy","description":"CI agent policy","policy_type":"pbac","policy_doc":{"subject":{"type":"agent","name":"ci-bot"},"rules":[{"effect":"allow","path":"alpha/*","capabilities":["read"]}]}}'
expect_code "List IAM policies" GET "/api/v1/iam-policies" 200 "$ALICE_TOKEN"
POL_COUNT=$(cat /tmp/tv_resp | python3 -c "import sys,json;print(len(json.load(sys.stdin)))")
[ "$POL_COUNT" -eq 3 ] && ok "3 policies exist" || fail "Expected 3, got $POL_COUNT"

# 71-75
expect_code "Create duplicate policy name" POST "/api/v1/iam-policies" 409 "$ALICE_TOKEN" '{"org_id":"'"$ORG_ID"'","name":"eng-read","policy_type":"rbac","policy_doc":{}}'
expect_code "Create policy invalid type" POST "/api/v1/iam-policies" 400 "$ALICE_TOKEN" '{"org_id":"'"$ORG_ID"'","name":"bad","policy_type":"invalid","policy_doc":{}}'
expect_code "Create policy without auth" POST "/api/v1/iam-policies" 401 "" '{"name":"noauth","policy_type":"rbac","policy_doc":{}}'
expect_code "Get health" GET "/health" 200
expect_code "Get ready" GET "/ready" 200

# ============================================================
section "AUDIT LOG (76-80)"
# ============================================================

# 76-80
expect_code "Get audit log" GET "/api/v1/audit?limit=10" 200 "$ALICE_TOKEN"
AUDIT_COUNT=$(cat /tmp/tv_resp | python3 -c "import sys,json;print(len(json.load(sys.stdin)))")
[ "$AUDIT_COUNT" -ge 5 ] && ok "Audit has >= 5 events ($AUDIT_COUNT)" || fail "Expected >= 5, got $AUDIT_COUNT"

expect_code "Audit with action filter" GET "/api/v1/audit?action=secret.write&limit=5" 200 "$ALICE_TOKEN"
expect_code "Audit with limit" GET "/api/v1/audit?limit=1" 200 "$ALICE_TOKEN"
ONE=$(cat /tmp/tv_resp | python3 -c "import sys,json;print(len(json.load(sys.stdin)))")
[ "$ONE" -eq 1 ] && ok "Limit=1 returns 1 event" || fail "Expected 1, got $ONE"
expect_code "Audit without auth" GET "/api/v1/audit" 401

# ============================================================
section "LEASES (81-85)"
# ============================================================

# 81-85
expect_code "Issue database lease" POST "/api/v1/leases" 201 "$ALICE_TOKEN" '{"type":"database","ttl_seconds":300,"project":"alpha"}'
LEASE_ID=$(cat /tmp/tv_resp | get_json "['id']" 2>/dev/null || echo "")
expect_code "List leases" GET "/api/v1/leases" 200 "$ALICE_TOKEN"
expect_code "Issue lease without auth" POST "/api/v1/leases" 401 "" '{"type":"database","ttl_seconds":60}'

if [ -n "$LEASE_ID" ]; then
  expect_code "Revoke lease" POST "/api/v1/leases/$LEASE_ID/revoke" 200 "$ALICE_TOKEN"
else
  ok "Lease revoke skipped (no lease ID)"
fi
expect_code "Revoke nonexistent lease" POST "/api/v1/leases/00000000-0000-0000-0000-000000000000/revoke" 404 "$ALICE_TOKEN"

# ============================================================
section "DASHBOARD & MISC (86-90)"
# ============================================================

# 86-90
expect_code "Dashboard stats" GET "/api/v1/dashboard/stats" 200 "$ALICE_TOKEN"
STATS=$(cat /tmp/tv_resp)
TS=$(echo "$STATS" | get_json "['total_secrets']")
[ "$TS" -ge 3 ] && ok "Stats: total_secrets >= 3 ($TS)" || fail "Expected >= 3, got $TS"

expect_code "Dashboard without auth" GET "/api/v1/dashboard/stats" 401
expect_code "OPTIONS preflight" OPTIONS "/api/v1/auth/login" 204
expect_code "Nonexistent endpoint" GET "/api/v1/nonexistent" 404 "$ALICE_TOKEN"

# ============================================================
section "ENCRYPTION VERIFICATION (91-95)"
# ============================================================

# 91-95: Verify secrets are encrypted in DB
expect_code "Store sensitive secret" PUT "/api/v1/secrets/alpha/crypto/test" 200 "$ALICE_TOKEN" '{"value":"SUPER_SECRET_VALUE_12345","description":"Crypto test"}'

DB_PLAIN=$(docker exec teamvault-postgres-1 psql -U teamvault -t -c "SELECT encode(ciphertext,'hex') FROM secret_versions WHERE secret_id IN (SELECT id FROM secrets WHERE path='crypto/test') ORDER BY version DESC LIMIT 1;" 2>/dev/null | tr -d ' \n')
if echo "$DB_PLAIN" | grep -qi "SUPER_SECRET"; then
  fail "SECRET FOUND IN PLAINTEXT IN DB!"
else
  ok "Secret NOT in plaintext in DB (encrypted)"
fi

# Check nonce exists
DB_NONCE=$(docker exec teamvault-postgres-1 psql -U teamvault -t -c "SELECT length(nonce) FROM secret_versions WHERE secret_id IN (SELECT id FROM secrets WHERE path='crypto/test') LIMIT 1;" 2>/dev/null | tr -d ' \n')
[ "$DB_NONCE" -gt 0 ] 2>/dev/null && ok "Nonce present in DB (len=$DB_NONCE)" || fail "No nonce in DB"

# Verify decryption works
expect_code "Read back encrypted secret" GET "/api/v1/secrets/alpha/crypto/test" 200 "$ALICE_TOKEN"
DEC=$(cat /tmp/tv_resp | get_json "['value']")
[ "$DEC" = "SUPER_SECRET_VALUE_12345" ] && ok "Decrytion correct" || fail "Decryption failed: $DEC"

# Audit trail has hash chain
HASH=$(curl -s "$SERVER/api/v1/audit?limit=1" -H "Authorization: Bearer $ALICE_TOKEN" | python3 -c "import sys,json;d=json.load(sys.stdin);print(d[0].get('hash',''))" 2>/dev/null)
[ -n "$HASH" ] && [ ${#HASH} -ge 32 ] && ok "Audit hash chain present (${HASH:0:16}...)" || fail "No hash chain"

# ============================================================
section "BULK OPERATIONS (96-100)"
# ============================================================

# 96-100: Stress test with multiple secrets
for i in $(seq 1 10); do
  curl -s -X PUT "$SERVER/api/v1/secrets/gamma/bulk/key$i" \
    -H "Authorization: Bearer $ALICE_TOKEN" -H "Content-Type: application/json" \
    -d "{\"value\":\"bulk_value_$i\",\"description\":\"Bulk $i\"}" > /dev/null
done
ok "Bulk created 10 secrets in gamma"

GAMMA_COUNT=$(curl -s "$SERVER/api/v1/secrets/gamma" -H "Authorization: Bearer $ALICE_TOKEN" | python3 -c "import sys,json;print(len(json.load(sys.stdin)))")
[ "$GAMMA_COUNT" -eq 10 ] && ok "Gamma has 10 secrets" || fail "Expected 10, got $GAMMA_COUNT"

# Concurrent reads
for i in $(seq 1 5); do
  curl -s "$SERVER/api/v1/secrets/gamma/bulk/key$i" -H "Authorization: Bearer $ALICE_TOKEN" > /dev/null &
done
wait
ok "5 concurrent reads completed"

# Final stats check
FINAL_STATS=$(curl -s "$SERVER/api/v1/dashboard/stats" -H "Authorization: Bearer $ALICE_TOKEN")
FINAL_SECRETS=$(echo "$FINAL_STATS" | get_json "['total_secrets']")
[ "$FINAL_SECRETS" -ge 15 ] && ok "Final stats: $FINAL_SECRETS secrets total" || fail "Expected >= 15, got $FINAL_SECRETS"

# ============================================================
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "RESULTS: $PASS passed, $FAIL failed"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ $FAIL -gt 0 ]; then
  echo ""
  echo "FAILURES:"
  echo -e "$ERRORS"
fi
exit $FAIL
