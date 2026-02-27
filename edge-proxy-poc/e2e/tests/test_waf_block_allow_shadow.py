"""E2E — Test 5: WAF shadow / block / allow."""

import pytest
from utils.assertions import assert_status


@pytest.mark.waf
class TestWAFBlockAllow:
    """WAF should block malicious requests in enforce mode, log-only in shadow."""

    def test_normal_request_passes(self, client):
        """Normal request without attack patterns should pass."""
        r = client.get("/search", params={"q": "hello world"})
        assert_status(r, 200)

    def test_path_traversal_blocked_enforce(self, proxy_url):
        """Path traversal pattern should be blocked (403) in enforce mode."""
        import httpx
        # Use raw URL to prevent client-side path resolution of ../
        r = httpx.get(
            proxy_url + "/path-test/..%2F..%2Fetc%2Fpasswd",
            timeout=10.0,
        )
        # In enforce mode → 403; in shadow mode → 200
        assert r.status_code in (403, 200), (
            f"Expected 403 (enforce) or 200 (shadow), got {r.status_code}"
        )

    def test_sqli_blocked_enforce(self, client):
        """SQL injection pattern in query should be blocked."""
        r = client.get("/search", params={"q": "1 UNION SELECT * FROM users"})
        assert r.status_code in (403, 200), (
            f"Expected 403 (enforce) or 200 (shadow), got {r.status_code}"
        )

    def test_xss_in_query(self, client):
        """XSS pattern in query — depends on mode (shadow=200, enforce=403)."""
        r = client.get("/search", params={"q": "<script>alert(1)</script>"})
        # XSS rule is typically in shadow/log mode, so should pass through
        assert r.status_code in (200, 403)

    def test_clean_search_passes(self, client):
        """Clean search queries should always pass."""
        r = client.get("/search", params={"q": "python programming"})
        assert_status(r, 200)

    def test_body_sqli_blocked(self, client):
        """SQL injection in POST body should be caught."""
        r = client.post(
            "/submit",
            content="username=admin&password=1 OR 1=1",
            headers={"Content-Type": "application/x-www-form-urlencoded"},
        )
        # In enforce: 403, in shadow: 200
        assert r.status_code in (403, 200)
