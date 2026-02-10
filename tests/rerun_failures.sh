#!/bin/bash
# Re-run the failed tests now that we have a policy allowing admin to manage secrets
BASE="http://localhost:8443"

# Get fresh admin token
TOKEN=$(curl -s -X POST "$BASE/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"email":"admin@test.com","password":"admin123"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
AUTH="Authorization: Bearer $TOKEN"

echo "=== Using admin token ==="
echo ""

# ============================================================
# TEST 9 (recheck): Path traversal with ..
# ============================================================
echo "=== TEST 9: Put secret with path containing .. ==="
resp=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X PUT "$BASE/api/v1/secrets/testproject/../etc/passwd" \
    -H "Content-Type: application/json" \
    -H "$AUTH" \
    -d '{"value":"evil"}')
echo "$resp"
echo ""

# Also try URL-encoded ..
echo "=== TEST 9b: Put secret with URL-encoded .. ==="
resp=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X PUT "$BASE/api/v1/secrets/testproject/%2e%2e/etc/passwd" \
    -H "Content-Type: application/json" \
    -H "$AUTH" \
    -d '{"value":"evil"}')
echo "$resp"
echo ""

# ============================================================
# TEST 10: Put secret with empty value
# ============================================================
echo "=== TEST 10: Put secret with empty value ==="
resp=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X PUT "$BASE/api/v1/secrets/testproject/empty-test" \
    -H "Content-Type: application/json" \
    -H "$AUTH" \
    -d '{"value":""}')
echo "$resp"
echo ""

# ============================================================
# TEST 11: Put secret with value larger than 1MB (use @file)
# ============================================================
echo "=== TEST 11: Put secret with value larger than 1MB ==="
python3 -c "import json; print(json.dumps({'value': 'x'*1048577}))" > /tmp/big_secret.json
resp=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X PUT "$BASE/api/v1/secrets/testproject/bigvalue" \
    -H "Content-Type: application/json" \
    -H "$AUTH" \
    -d @/tmp/big_secret.json --max-time 30)
echo "$resp"
rm -f /tmp/big_secret.json
echo ""

# ============================================================
# TEST 12: Put secret with unicode value
# ============================================================
echo "=== TEST 12: Put secret with unicode value ==="
resp=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X PUT "$BASE/api/v1/secrets/testproject/unicode-test" \
    -H "Content-Type: application/json" \
    -H "$AUTH" \
    -d '{"value":"🔑こんにちは世界🌍 émojis and ñ"}')
echo "PUT: $resp"
echo ""

echo "=== TEST 12b: Read back unicode value ==="
resp=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X GET "$BASE/api/v1/secrets/testproject/unicode-test" \
    -H "$AUTH")
echo "GET: $resp"
echo ""

# ============================================================
# TEST 13: Get secret that was soft-deleted
# ============================================================
echo "=== TEST 13: Store, delete, then get secret ==="
echo "--- Creating ---"
resp=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X PUT "$BASE/api/v1/secrets/testproject/delete-me2" \
    -H "Content-Type: application/json" \
    -H "$AUTH" \
    -d '{"value":"temporary"}')
echo "$resp"
echo "--- Deleting ---"
resp=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X DELETE "$BASE/api/v1/secrets/testproject/delete-me2" \
    -H "$AUTH")
echo "$resp"
echo "--- Reading deleted ---"
resp=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X GET "$BASE/api/v1/secrets/testproject/delete-me2" \
    -H "$AUTH")
echo "$resp"
echo ""

# ============================================================
# TEST 29: Update a secret 100 times rapidly
# ============================================================
echo "=== TEST 29: Version stress (100 rapid updates) ==="
# First create the secret
curl -s -X PUT "$BASE/api/v1/secrets/testproject/version-stress2" \
    -H "Content-Type: application/json" \
    -H "$AUTH" \
    -d '{"value":"v0"}' > /dev/null

error_count=0
for i in $(seq 1 100); do
    code=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$BASE/api/v1/secrets/testproject/version-stress2" \
        -H "Content-Type: application/json" \
        -H "$AUTH" \
        -d "{\"value\":\"version-$i\"}")
    if [ "$code" -ge 400 ]; then
        error_count=$((error_count + 1))
        echo "  Error at iteration $i: HTTP $code"
    fi
done
echo "Errors: $error_count/100"

echo "--- Checking versions ---"
resp=$(curl -s "$BASE/api/v1/secrets/testproject/version-stress2/versions" -H "$AUTH")
count=$(echo "$resp" | python3 -c "import sys,json; data=json.load(sys.stdin); print(len(data) if isinstance(data,list) else 'not a list')" 2>&1)
echo "Version count: $count"
echo ""

# ============================================================
# TEST 32: Store secret with newlines and tabs
# ============================================================
echo "=== TEST 32: Store secret with newlines and tabs ==="
resp=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X PUT "$BASE/api/v1/secrets/testproject/whitespace-test" \
    -H "Content-Type: application/json" \
    -H "$AUTH" \
    -d '{"value":"line1\nline2\ttab\nline3"}')
echo "PUT: $resp"

resp=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X GET "$BASE/api/v1/secrets/testproject/whitespace-test" \
    -H "$AUTH")
echo "GET: $resp"
echo ""

echo "=== ALL RERUNS COMPLETE ==="
