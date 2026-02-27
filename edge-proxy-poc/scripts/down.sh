#!/usr/bin/env bash
set -euo pipefail
echo "=== Stopping OpenFlare Edge Proxy POC ==="
docker compose down "$@"
echo "=== Services stopped ==="
