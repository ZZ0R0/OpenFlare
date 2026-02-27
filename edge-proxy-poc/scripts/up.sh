#!/usr/bin/env bash
set -euo pipefail
echo "=== Starting OpenFlare Edge Proxy POC ==="
docker compose up --build -d
echo ""
echo "=== Waiting for services to be healthy ==="
docker compose ps
echo ""
echo "=== Services started ==="
echo "  Proxy:    http://localhost:8080"
echo "  Admin:    http://localhost:8081"
echo "  Metrics:  http://localhost:8080/metrics"
echo ""
echo "Run 'make test-e2e' to execute E2E tests"
echo "Run 'make logs' to follow proxy logs"
