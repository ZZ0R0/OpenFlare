"""E2E — Test 7: Upstream timeout handling."""

import pytest
from utils.assertions import assert_status


@pytest.mark.robustness
class TestUpstreamTimeout:
    """Proxy should handle slow/failing upstreams gracefully."""

    def test_slow_endpoint_timeout(self, client):
        """Request to /slow with delay > proxy timeout → controlled error (504)."""
        # response_header_timeout_ms is 3000 in test config
        r = client.get("/slow", params={"delay_ms": "10000"}, timeout=15.0)
        assert r.status_code in (502, 504), (
            f"Expected 502/504 for upstream timeout, got {r.status_code}"
        )

    def test_slow_endpoint_within_timeout(self, client):
        """Request to /slow within timeout should succeed."""
        r = client.get("/slow", params={"delay_ms": "500"}, timeout=10.0)
        assert_status(r, 200)

    def test_upstream_error_codes_forwarded(self, client):
        """Upstream 4xx/5xx should be forwarded."""
        r = client.get("/error/500")
        assert r.status_code == 500

        r = client.get("/error/404")
        assert r.status_code == 404

        r = client.get("/error/503")
        assert r.status_code == 503
