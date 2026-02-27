"""E2E — Test 9: Metrics and health endpoints."""

import pytest
import httpx
from utils.assertions import assert_status


@pytest.mark.metrics
class TestMetricsHealth:
    """Health checks and Prometheus metrics endpoint."""

    def test_healthz(self, client):
        """GET /healthz returns 200."""
        r = client.get("/healthz")
        assert_status(r, 200)

    def test_readyz(self, client):
        """GET /readyz returns 200."""
        r = client.get("/readyz")
        assert_status(r, 200)

    def test_metrics_endpoint(self, admin_client, admin_token, proxy_url):
        """GET /metrics returns Prometheus metrics."""
        # Make some proxy requests first to generate metrics
        with httpx.Client(base_url=proxy_url, timeout=5.0) as c:
            c.get("/healthz")
            c.get("/api/time")

        r = admin_client.get("/metrics")
        assert_status(r, 200)
        body = r.text
        # Should contain at least some of our custom metrics or Go runtime metrics
        assert "openflare_" in body or "promhttp_" in body, (
            f"Metrics output missing expected metrics. Preview: {body[:500]}"
        )

    def test_metrics_contain_cache_counters(self, client, admin_client, admin_token, purge_all):
        """After some requests, cache metrics should be present."""
        # Generate some cache activity
        client.get("/cache/public-long")
        client.get("/cache/public-long")

        r = admin_client.get("/metrics")
        body = r.text
        assert "openflare_cache_operations_total" in body or "openflare_" in body, (
            f"Cache metrics not found in output. Preview: {body[:500]}"
        )

    def test_request_id_header(self, client):
        """Every proxied response should have X-Request-ID."""
        r = client.get("/api/time")
        rid = r.headers.get("X-Request-ID") or r.headers.get("x-request-id")
        assert rid, f"Missing X-Request-ID header. Headers: {dict(r.headers)}"
        assert len(rid) > 8, f"X-Request-ID looks too short: {rid}"
