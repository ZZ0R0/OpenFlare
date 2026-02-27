#!/usr/bin/env bash
set -euo pipefail

PROXY_URL="${PROXY_URL:-http://localhost:8080}"
ADMIN_URL="${ADMIN_URL:-http://localhost:8081}"
ADMIN_TOKEN="${ADMIN_TOKEN:-change-me}"

echo "=== OpenFlare Smoke Test ==="
echo ""

PASS=0
FAIL=0

check() {
    local desc="$1"
    local url="$2"
    local expected_code="$3"

    code=$(curl -s -o /dev/null -w "%{http_code}" "$url" 2>/dev/null || echo "000")
    if [ "$code" = "$expected_code" ]; then
        echo "  ✓ $desc (HTTP $code)"
        PASS=$((PASS + 1))
    else
        echo "  ✗ $desc (expected $expected_code, got $code)"
        FAIL=$((FAIL + 1))
    fi
}

echo "--- Health Checks ---"
check "Proxy healthz" "$PROXY_URL/healthz" "200"
check "Proxy readyz" "$PROXY_URL/readyz" "200"

echo ""
echo "--- Proxy Routing ---"
check "Homepage" "$PROXY_URL/" "200"
check "Static JS" "$PROXY_URL/static/app.v1.js" "200"
check "API time" "$PROXY_URL/api/time" "200"

echo ""
echo "--- Cache ---"
# First request = MISS, second = HIT
curl -s "$PROXY_URL/static/app.v1.js" > /dev/null
CACHE_STATUS=$(curl -s -D - "$PROXY_URL/static/app.v1.js" -o /dev/null 2>&1 | grep -i "X-Edge-Cache" | tr -d '\r' | awk '{print $2}')
if [ "$CACHE_STATUS" = "HIT" ]; then
    echo "  ✓ Cache HIT on second request"
    PASS=$((PASS + 1))
else
    echo "  ✗ Expected cache HIT, got: $CACHE_STATUS"
    FAIL=$((FAIL + 1))
fi

echo ""
echo "--- Admin API ---"
check "Cache stats (with auth)" "$ADMIN_URL/admin/cache/stats" "200"

echo ""
echo "--- Metrics ---"
check "Prometheus metrics" "$PROXY_URL/metrics" "200"

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
