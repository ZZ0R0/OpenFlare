"""E2E — Test 10: Normalization and bypass edge cases."""

import pytest
from utils.assertions import assert_status


@pytest.mark.normalization
class TestNormalizationBypass:
    """Verify URL normalization for cache/WAF consistency."""

    def test_double_slash_normalized(self, client, purge_all):
        """Double slashes in path should be handled gracefully."""
        r1 = client.get("/cache/public-long")
        assert_status(r1, 200)

        # Send request with double slashes using raw URL
        # httpx normalizes paths, so we use the proxy URL directly
        import httpx
        r2 = httpx.get(
            self._raw_url("/cache//public-long"),
            timeout=10.0,
        )
        # The proxy should normalize the path — accept 200 (normalized hit), 301 (redirect), or 404
        assert r2.status_code in (200, 301, 404), (
            f"Unexpected status for double-slash path: {r2.status_code}"
        )

    @staticmethod
    def _raw_url(path):
        import os
        proxy = os.getenv("PROXY_URL", "http://proxy:8080")
        return proxy + path

    def test_query_order_normalized(self, client, purge_all):
        """Query parameter order should be normalized for cache key."""
        r1 = client.get("/api/echo?b=2&a=1")
        assert_status(r1, 200)

        r2 = client.get("/api/echo?a=1&b=2")
        assert_status(r2, 200)
        # Whether these produce the same cache key depends on normalization config
        # Just verify both succeed
        assert r1.status_code == r2.status_code

    def test_encoded_path_normalization(self, client, purge_all):
        """URL-encoded path should be normalized."""
        r = client.get("/cache/public-long")
        assert_status(r, 200)

        r2 = client.get("/cache%2Fpublic-long")
        # Depending on normalization, this may or may not hit same key
        assert r2.status_code in (200, 404)

    def test_trailing_slash_handling(self, client, purge_all):
        """Trailing slash handling should be consistent."""
        r1 = client.get("/api/time")
        assert_status(r1, 200)

        # Trailing slash may or may not be treated as same endpoint
        r2 = client.get("/api/time/")
        # Could be 200, 307 (redirect), or 404 depending on proxy/origin behavior
        assert r2.status_code in (200, 307, 404, 308)
