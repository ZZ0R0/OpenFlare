#!/usr/bin/env bash
set -euo pipefail
echo "=== Running E2E Tests ==="
docker compose run --rm e2e pytest -v --tb=short "$@"
echo ""
echo "=== E2E Tests Complete ==="
