"""E2E — Test 6: Rate limiting."""

import time
import pytest
import httpx
from utils.assertions import assert_status


@pytest.mark.ratelimit
class TestRateLimit:
    """Rate limiter should return 429 after burst exceeds threshold."""

    def test_burst_triggers_429(self, proxy_url):
        """Send a burst of requests; some should get 429."""
        got_429 = False
        got_200 = False
        results = []

        with httpx.Client(base_url=proxy_url, timeout=5.0) as c:
            for i in range(60):
                try:
                    r = c.get("/api/time")
                    results.append(r.status_code)
                    if r.status_code == 429:
                        got_429 = True
                    elif r.status_code == 200:
                        got_200 = True
                except httpx.ReadTimeout:
                    results.append("timeout")

        assert got_200, f"Expected at least one 200 response. Results: {results[:20]}"
        assert got_429, (
            f"Expected at least one 429 response in burst of 60. "
            f"Results: {results[:20]}... "
            f"(200s: {results.count(200)}, 429s: {results.count(429)})"
        )

    def test_recovery_after_rate_limit(self, proxy_url):
        """After being rate-limited, requests should succeed after cooldown."""
        with httpx.Client(base_url=proxy_url, timeout=5.0) as c:
            # Exhaust rate limit
            for _ in range(60):
                c.get("/api/time")

            # Wait for rate limit window to reset
            time.sleep(2)

            # Should succeed again
            r = c.get("/api/time")
            assert_status(r, 200)

    def test_429_has_retry_after(self, proxy_url):
        """429 response should include Retry-After header."""
        with httpx.Client(base_url=proxy_url, timeout=5.0) as c:
            for _ in range(60):
                r = c.get("/api/time")
                if r.status_code == 429:
                    assert "Retry-After" in r.headers or "retry-after" in r.headers, (
                        f"429 response missing Retry-After. Headers: {dict(r.headers)}"
                    )
                    return
        pytest.skip("Could not trigger 429 — rate limit may be high")
