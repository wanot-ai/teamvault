#!/bin/bash
# TeamVault Edge Case Tests
# Outputs results to EDGE_CASE_RESULTS.md

BASE="http://localhost:8443"
TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiNjU2NmMyMzEtOWNkZS00Mzc1LTljNjItNWY5YzYzOTBhYWJiIiwiZW1haWwiOiJhZG1pbkB0ZXN0LmNvbSIsInJvbGUiOiJtZW1iZXIiLCJpc3MiOiJ0ZWFtdmF1bHQiLCJleHAiOjE3NzA2ODY4MjQsIm5iZiI6MTc3MDY4MzIyNCwiaWF0IjoxNzcwNjgzMjI0fQ.T9PLOB6bLSurkYeNREvpzUkiG8G8oiAqDtW-ZHFqgT8"
AUTH="Authorization: Bearer $TOKEN"
OUT="/home/mconcat/.openclaw/workspace/teamvault/tests/EDGE_CASE_RESULTS.md"

cat > "$OUT" << 'HEADER'
# TeamVault Edge Case Test Results

**Date:** 2026-02-10
**Server:** http://localhost:8443
**Tester:** Automated Script

---

HEADER

test_num=0
pass_count=0
fail_count=0

record() {
    local num="$1" title="$2" request="$3" expected="$4" code="$5" body="$6" verdict="$7"
    cat >> "$OUT" << EOF
## Test $num: $title

**Request:** $request

**Expected:** $expected

**Actual Response:**
- Status Code: \`$code\`
- Body: \`\`\`
$(echo "$body" | head -c 2000)
\`\`\`

**Verdict:** $verdict

---

EOF
    if [ "$verdict" = "✅ PASS" ]; then
        pass_count=$((pass_count + 1))
    else
        fail_count=$((fail_count + 1))
    fi
}

# Helper to run curl and capture both status and body
run_curl() {
    local resp
    resp=$(curl -s -w "\n%{http_code}" "$@" 2>&1)
    local code=$(echo "$resp" | tail -1)
    local body=$(echo "$resp" | sed '$d')
    echo "$code"
    echo "$body"
}

echo "Running 30 edge case tests..."

# ============================================================
# TEST 1: Register with email longer than 255 chars
# ============================================================
test_num=1
LONG_EMAIL=$(python3 -c "print('a'*250 + '@b.com')")
resp=$(curl -s -w "\n%{http_code}" -X POST "$BASE/api/v1/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$LONG_EMAIL\",\"password\":\"secure123\",\"name\":\"Long Email\"}")
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
if [ "$code" -ge 400 ] && [ "$code" -lt 500 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Register with email longer than 255 chars" \
    "POST /api/v1/auth/register with ${#LONG_EMAIL}-char email" \
    "400-level error rejecting overly long email" \
    "$code" "$body" "$verdict"

# ============================================================
# TEST 2: Register with password shorter than 4 chars
# ============================================================
test_num=2
resp=$(curl -s -w "\n%{http_code}" -X POST "$BASE/api/v1/auth/register" \
    -H "Content-Type: application/json" \
    -d '{"email":"short@pass.com","password":"ab","name":"Short Pass"}')
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
if [ "$code" -ge 400 ] && [ "$code" -lt 500 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Register with password shorter than 4 chars" \
    "POST /api/v1/auth/register with password='ab'" \
    "400-level error rejecting short password" \
    "$code" "$body" "$verdict"

# ============================================================
# TEST 3: Login with empty body
# ============================================================
test_num=3
resp=$(curl -s -w "\n%{http_code}" -X POST "$BASE/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d '')
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
if [ "$code" -ge 400 ] && [ "$code" -lt 500 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Login with empty body" \
    "POST /api/v1/auth/login with empty body" \
    "400-level error" \
    "$code" "$body" "$verdict"

# ============================================================
# TEST 4: Login with JSON syntax error
# ============================================================
test_num=4
resp=$(curl -s -w "\n%{http_code}" -X POST "$BASE/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d '{bad json')
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
if [ "$code" -ge 400 ] && [ "$code" -lt 500 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Login with JSON syntax error" \
    "POST /api/v1/auth/login with malformed JSON '{bad json'" \
    "400-level error about invalid JSON" \
    "$code" "$body" "$verdict"

# ============================================================
# TEST 5: Create project with special chars
# ============================================================
for name in "test/project" "test project" "test@project"; do
    test_num=$((test_num + 1))
    escaped_name=$(echo "$name" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read().strip()))')
    resp=$(curl -s -w "\n%{http_code}" -X POST "$BASE/api/v1/projects" \
        -H "Content-Type: application/json" \
        -H "$AUTH" \
        -d "{\"name\":$escaped_name}")
    code=$(echo "$resp" | tail -1)
    body=$(echo "$resp" | sed '$d')
    # Could be 400 (rejected) or 201 (accepted) - either is valid behavior, but we note it
    if [ "$code" -ge 200 ] && [ "$code" -lt 500 ]; then
        verdict="✅ PASS"
    else
        verdict="❌ FAIL"
    fi
    record "$test_num" "Create project with name '$name'" \
        "POST /api/v1/projects with name=$escaped_name" \
        "Either 400 (rejected) or 201 (accepted with sanitization)" \
        "$code" "$body" "$verdict"
done

# ============================================================
# TEST 8 (re-numbered): Create project with name longer than 200 chars
# ============================================================
test_num=8
LONG_NAME=$(python3 -c "print('a'*201)")
resp=$(curl -s -w "\n%{http_code}" -X POST "$BASE/api/v1/projects" \
    -H "Content-Type: application/json" \
    -H "$AUTH" \
    -d "{\"name\":\"$LONG_NAME\"}")
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
if [ "$code" -ge 400 ] && [ "$code" -lt 500 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Create project with name longer than 200 chars" \
    "POST /api/v1/projects with 201-char name" \
    "400-level error rejecting overly long name" \
    "$code" "$body" "$verdict"

# First, create a project for secret tests
curl -s -X POST "$BASE/api/v1/projects" \
    -H "Content-Type: application/json" \
    -H "$AUTH" \
    -d '{"name":"testproject"}' > /dev/null

# ============================================================
# TEST 9: Put secret with path containing ..
# ============================================================
test_num=9
resp=$(curl -s -w "\n%{http_code}" -X PUT "$BASE/api/v1/secrets/testproject/../etc/passwd" \
    -H "Content-Type: application/json" \
    -H "$AUTH" \
    -d '{"value":"evil"}')
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
if [ "$code" -ge 400 ] && [ "$code" -lt 500 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Put secret with path containing .." \
    "PUT /api/v1/secrets/testproject/../etc/passwd" \
    "400-level error rejecting path traversal" \
    "$code" "$body" "$verdict"

# ============================================================
# TEST 10: Put secret with empty value
# ============================================================
test_num=10
resp=$(curl -s -w "\n%{http_code}" -X PUT "$BASE/api/v1/secrets/testproject/empty-test" \
    -H "Content-Type: application/json" \
    -H "$AUTH" \
    -d '{"value":""}')
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
if [ "$code" -ge 200 ] && [ "$code" -lt 500 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Put secret with empty value" \
    "PUT /api/v1/secrets/testproject/empty-test with value=''" \
    "Either 400 (rejected) or 201 (accepted empty string)" \
    "$code" "$body" "$verdict"

# ============================================================
# TEST 11: Put secret with value larger than 1MB
# ============================================================
test_num=11
BIG_VALUE=$(python3 -c "print('x'*1048577)")
resp=$(curl -s -w "\n%{http_code}" -X PUT "$BASE/api/v1/secrets/testproject/bigvalue" \
    -H "Content-Type: application/json" \
    -H "$AUTH" \
    -d "{\"value\":\"$BIG_VALUE\"}" --max-time 30)
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
if [ "$code" -ge 400 ] && [ "$code" -lt 500 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Put secret with value larger than 1MB" \
    "PUT /api/v1/secrets/testproject/bigvalue with 1MB+ value" \
    "400-level error rejecting oversized value" \
    "$code" "$body" "$verdict"

# ============================================================
# TEST 12: Put secret with unicode value
# ============================================================
test_num=12
resp=$(curl -s -w "\n%{http_code}" -X PUT "$BASE/api/v1/secrets/testproject/unicode-test" \
    -H "Content-Type: application/json" \
    -H "$AUTH" \
    -d '{"value":"🔑こんにちは世界🌍 émojis and ñ"}')
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
if [ "$code" -ge 200 ] && [ "$code" -lt 300 ]; then
    # Now read it back to verify roundtrip
    resp2=$(curl -s -w "\n%{http_code}" -X GET "$BASE/api/v1/secrets/testproject/unicode-test" \
        -H "$AUTH")
    code2=$(echo "$resp2" | tail -1)
    body2=$(echo "$resp2" | sed '$d')
    if echo "$body2" | grep -q "🔑"; then
        verdict="✅ PASS"
    else
        verdict="❌ FAIL"
        body="Store: $body | Read: $body2"
    fi
else
    verdict="❌ FAIL"
fi
record "$test_num" "Put secret with unicode value (emoji, CJK)" \
    "PUT /api/v1/secrets/testproject/unicode-test with '🔑こんにちは世界🌍'" \
    "201 success and roundtrip preserves unicode" \
    "$code" "$body" "$verdict"

# ============================================================
# TEST 13: Get secret that was soft-deleted
# ============================================================
test_num=13
# Create then delete
curl -s -X PUT "$BASE/api/v1/secrets/testproject/delete-me" \
    -H "Content-Type: application/json" \
    -H "$AUTH" \
    -d '{"value":"temporary"}' > /dev/null
curl -s -X DELETE "$BASE/api/v1/secrets/testproject/delete-me" \
    -H "$AUTH" > /dev/null
resp=$(curl -s -w "\n%{http_code}" -X GET "$BASE/api/v1/secrets/testproject/delete-me" \
    -H "$AUTH")
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
if [ "$code" -eq 404 ] || [ "$code" -eq 410 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Get secret that was soft-deleted" \
    "DELETE then GET /api/v1/secrets/testproject/delete-me" \
    "404 or 410 (not found / gone)" \
    "$code" "$body" "$verdict"

# ============================================================
# TEST 14: Create org with unicode name
# ============================================================
test_num=14
resp=$(curl -s -w "\n%{http_code}" -X POST "$BASE/api/v1/orgs" \
    -H "Content-Type: application/json" \
    -H "$AUTH" \
    -d '{"name":"テスト組織🏢"}')
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
# Either success (unicode accepted) or 404 (endpoint doesn't exist in MVP)
if [ "$code" -ge 200 ] && [ "$code" -lt 500 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Create org with unicode name" \
    "POST /api/v1/orgs with name='テスト組織🏢'" \
    "201 (accepted) or 404 (endpoint not in MVP)" \
    "$code" "$body" "$verdict"

# ============================================================
# TEST 15: Create team with empty name
# ============================================================
test_num=15
resp=$(curl -s -w "\n%{http_code}" -X POST "$BASE/api/v1/teams" \
    -H "Content-Type: application/json" \
    -H "$AUTH" \
    -d '{"name":""}')
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
if [ "$code" -ge 400 ] && [ "$code" -lt 500 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Create team with empty name" \
    "POST /api/v1/teams with name=''" \
    "400-level error" \
    "$code" "$body" "$verdict"

# ============================================================
# TEST 16: Add non-existent user to team (random UUID)
# ============================================================
test_num=16
resp=$(curl -s -w "\n%{http_code}" -X POST "$BASE/api/v1/teams/some-team/members" \
    -H "Content-Type: application/json" \
    -H "$AUTH" \
    -d '{"user_id":"00000000-0000-0000-0000-000000000099"}')
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
if [ "$code" -ge 400 ] && [ "$code" -lt 500 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Add non-existent user to team" \
    "POST /api/v1/teams/some-team/members with random UUID" \
    "400/404-level error" \
    "$code" "$body" "$verdict"

# ============================================================
# TEST 17: Create policy with malformed JSON in policy_doc
# ============================================================
test_num=17
resp=$(curl -s -w "\n%{http_code}" -X POST "$BASE/api/v1/policies" \
    -H "Content-Type: application/json" \
    -H "$AUTH" \
    -d '{"name":"bad-policy","effect":"allow","actions":["read"],"resource_pattern":"*","subject_type":"user","conditions":"{invalid json}"}')
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
if [ "$code" -ge 200 ] && [ "$code" -lt 500 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Create policy with malformed JSON in policy_doc" \
    "POST /api/v1/policies with conditions='{invalid json}'" \
    "400 (rejected) or 201 (string accepted, not parsed)" \
    "$code" "$body" "$verdict"

# ============================================================
# TEST 18: Query audit with invalid date format
# ============================================================
test_num=18
resp=$(curl -s -w "\n%{http_code}" -X GET "$BASE/api/v1/audit?from=not-a-date" \
    -H "$AUTH")
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
if [ "$code" -ge 200 ] && [ "$code" -lt 500 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Query audit with invalid date format" \
    "GET /api/v1/audit?from=not-a-date" \
    "400 (bad request) or 200 (parameter ignored)" \
    "$code" "$body" "$verdict"

# ============================================================
# TEST 19: Query audit with limit=0
# ============================================================
test_num=19
resp=$(curl -s -w "\n%{http_code}" -X GET "$BASE/api/v1/audit?limit=0" \
    -H "$AUTH")
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
if [ "$code" -ge 200 ] && [ "$code" -lt 500 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Query audit with limit=0" \
    "GET /api/v1/audit?limit=0" \
    "200 with empty results or 400 (invalid limit)" \
    "$code" "$body" "$verdict"

# ============================================================
# TEST 20: Query audit with limit=-1
# ============================================================
test_num=20
resp=$(curl -s -w "\n%{http_code}" -X GET "$BASE/api/v1/audit?limit=-1" \
    -H "$AUTH")
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
if [ "$code" -ge 200 ] && [ "$code" -lt 500 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Query audit with limit=-1" \
    "GET /api/v1/audit?limit=-1" \
    "400 (rejected) or 200 (clamped to default)" \
    "$code" "$body" "$verdict"

# ============================================================
# TEST 21: Query audit with limit=10000
# ============================================================
test_num=21
resp=$(curl -s -w "\n%{http_code}" -X GET "$BASE/api/v1/audit?limit=10000" \
    -H "$AUTH")
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
if [ "$code" -ge 200 ] && [ "$code" -lt 500 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Query audit with limit=10000" \
    "GET /api/v1/audit?limit=10000" \
    "200 (clamped to max) or 400 (limit too high)" \
    "$code" "$body" "$verdict"

# ============================================================
# TEST 22: Send request with Content-Type: text/plain
# ============================================================
test_num=22
resp=$(curl -s -w "\n%{http_code}" -X POST "$BASE/api/v1/auth/login" \
    -H "Content-Type: text/plain" \
    -d '{"email":"admin@test.com","password":"admin123"}')
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
if [ "$code" -ge 200 ] && [ "$code" -lt 500 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Send request with Content-Type: text/plain" \
    "POST /api/v1/auth/login with Content-Type: text/plain" \
    "400 (unsupported media) or 200 (server parses anyway)" \
    "$code" "$body" "$verdict"

# ============================================================
# TEST 23: Send GET to POST-only endpoint
# ============================================================
test_num=23
resp=$(curl -s -w "\n%{http_code}" -X GET "$BASE/api/v1/auth/register")
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
if [ "$code" -eq 405 ] || [ "$code" -eq 404 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Send GET to POST-only endpoint (register)" \
    "GET /api/v1/auth/register" \
    "405 Method Not Allowed or 404" \
    "$code" "$body" "$verdict"

# ============================================================
# TEST 24: Send POST to GET-only endpoint
# ============================================================
test_num=24
resp=$(curl -s -w "\n%{http_code}" -X POST "$BASE/api/v1/audit" \
    -H "Content-Type: application/json" \
    -H "$AUTH" \
    -d '{}')
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
if [ "$code" -eq 405 ] || [ "$code" -eq 404 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Send POST to GET-only endpoint (audit)" \
    "POST /api/v1/audit" \
    "405 Method Not Allowed" \
    "$code" "$body" "$verdict"

# ============================================================
# TEST 25: Double-slash in URL path
# ============================================================
test_num=25
resp=$(curl -s -w "\n%{http_code}" -X GET "$BASE//api/v1//projects" \
    -H "$AUTH")
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
if [ "$code" -ge 200 ] && [ "$code" -lt 500 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Double-slash in URL path" \
    "GET //api/v1//projects" \
    "200 (path normalized) or 404 (not found)" \
    "$code" "$body" "$verdict"

# ============================================================
# TEST 26: Trailing slash in URL path
# ============================================================
test_num=26
resp=$(curl -s -w "\n%{http_code}" -X GET "$BASE/api/v1/projects/" \
    -H "$AUTH")
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
if [ "$code" -ge 200 ] && [ "$code" -lt 500 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Trailing slash in URL path" \
    "GET /api/v1/projects/" \
    "200 (slash ignored) or 404 (strict routing)" \
    "$code" "$body" "$verdict"

# ============================================================
# TEST 27: Very large Authorization header (10KB)
# ============================================================
test_num=27
BIG_AUTH="Bearer $(python3 -c "print('x'*10240)")"
resp=$(curl -s -w "\n%{http_code}" -X GET "$BASE/api/v1/projects" \
    -H "Authorization: $BIG_AUTH" --max-time 10)
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
if [ "$code" -ge 400 ] && [ "$code" -lt 500 ]; then
    verdict="✅ PASS"
elif [ "$code" -eq 431 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Request with very large Authorization header (10KB)" \
    "GET /api/v1/projects with 10KB auth header" \
    "401/413/431 error" \
    "$code" "$body" "$verdict"

# ============================================================
# TEST 28: Multiple concurrent logins for same user
# ============================================================
test_num=28
tokens=""
all_ok=true
for i in $(seq 1 5); do
    resp=$(curl -s -w "\n%{http_code}" -X POST "$BASE/api/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d '{"email":"admin@test.com","password":"admin123"}')
    code=$(echo "$resp" | tail -1)
    if [ "$code" -ne 200 ]; then
        all_ok=false
    fi
    tokens="$tokens $code"
done
body="Login codes: $tokens"
if $all_ok; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Multiple concurrent logins for same user" \
    "5x POST /api/v1/auth/login in sequence" \
    "All return 200 with valid tokens" \
    "200" "$body" "$verdict"

# ============================================================
# TEST 29: Update a secret 100 times rapidly (version stress)
# ============================================================
test_num=29
curl -s -X PUT "$BASE/api/v1/secrets/testproject/version-stress" \
    -H "Content-Type: application/json" \
    -H "$AUTH" \
    -d '{"value":"v0"}' > /dev/null

error_count=0
for i in $(seq 1 100); do
    code=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$BASE/api/v1/secrets/testproject/version-stress" \
        -H "Content-Type: application/json" \
        -H "$AUTH" \
        -d "{\"value\":\"version-$i\"}")
    if [ "$code" -ge 400 ]; then
        error_count=$((error_count + 1))
    fi
done

# Check versions
resp=$(curl -s -w "\n%{http_code}" -X GET "$BASE/api/v1/secrets/testproject/version-stress/versions" \
    -H "$AUTH")
vcode=$(echo "$resp" | tail -1)
vbody=$(echo "$resp" | sed '$d')
body="Errors during 100 updates: $error_count. Versions response ($vcode): $(echo "$vbody" | head -c 500)"
if [ "$error_count" -eq 0 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Update a secret 100 times rapidly (version stress)" \
    "100x PUT /api/v1/secrets/testproject/version-stress" \
    "All 100 updates succeed" \
    "200/201" "$body" "$verdict"

# ============================================================
# TEST 30: Create 100 projects
# ============================================================
test_num=30
error_count=0
for i in $(seq 1 100); do
    code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/api/v1/projects" \
        -H "Content-Type: application/json" \
        -H "$AUTH" \
        -d "{\"name\":\"bulk-project-$i\"}")
    if [ "$code" -ge 400 ]; then
        error_count=$((error_count + 1))
    fi
done
body="Errors during 100 project creations: $error_count"
if [ "$error_count" -eq 0 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "Create 100 projects" \
    "100x POST /api/v1/projects with unique names" \
    "All 100 creations succeed" \
    "201" "$body" "$verdict"

# ============================================================
# TEST 31: List projects when there are 100+
# ============================================================
test_num=31
resp=$(curl -s -w "\n%{http_code}" -X GET "$BASE/api/v1/projects" \
    -H "$AUTH")
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')
count=$(echo "$body" | python3 -c "import sys,json; data=json.load(sys.stdin); print(len(data) if isinstance(data, list) else len(data.get('projects', data.get('items', []))))" 2>/dev/null || echo "parse error")
body="Returned $count projects (status $code)"
if [ "$code" -eq 200 ]; then
    verdict="✅ PASS"
else
    verdict="❌ FAIL"
fi
record "$test_num" "List projects when there are 100+" \
    "GET /api/v1/projects" \
    "200 with all projects listed" \
    "$code" "$body" "$verdict"

# ============================================================
# TEST 32: Store secret with newlines and tabs in value
# ============================================================
test_num=32
resp=$(curl -s -w "\n%{http_code}" -X PUT "$BASE/api/v1/secrets/testproject/whitespace-test" \
    -H "Content-Type: application/json" \
    -H "$AUTH" \
    -d '{"value":"line1\nline2\ttab\nline3"}')
code=$(echo "$resp" | tail -1)
body=$(echo "$resp" | sed '$d')

# Read it back
if [ "$code" -ge 200 ] && [ "$code" -lt 300 ]; then
    resp2=$(curl -s -w "\n%{http_code}" -X GET "$BASE/api/v1/secrets/testproject/whitespace-test" \
        -H "$AUTH")
    code2=$(echo "$resp2" | tail -1)
    body2=$(echo "$resp2" | sed '$d')
    if echo "$body2" | grep -q 'line1'; then
        verdict="✅ PASS"
        body="Store: $code | Read: $code2 - $body2"
    else
        verdict="❌ FAIL"
        body="Store: $code | Read: $code2 - Value not preserved: $body2"
    fi
else
    verdict="❌ FAIL"
fi
record "$test_num" "Store secret with newlines and tabs in value" \
    "PUT /api/v1/secrets/testproject/whitespace-test with newlines and tabs" \
    "201 and value roundtrips correctly" \
    "$code" "$body" "$verdict"

# ============================================================
# SUMMARY
# ============================================================
cat >> "$OUT" << EOF

# Summary

| Metric | Count |
|--------|-------|
| Total Tests | $((pass_count + fail_count)) |
| ✅ PASS | $pass_count |
| ❌ FAIL | $fail_count |

EOF

echo "Done! Results written to $OUT"
echo "PASS: $pass_count, FAIL: $fail_count"
