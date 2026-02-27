#!/usr/bin/env bash
set -euo pipefail

ADMIN_URL="${ADMIN_URL:-http://localhost:8081}"
ADMIN_TOKEN="${ADMIN_TOKEN:-change-me}"

echo "=== Purging all cache ==="
curl -s -X POST "$ADMIN_URL/admin/cache/purge-all" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | python3 -m json.tool 2>/dev/null || echo "Done"
echo ""
echo "=== Cache purged ==="
