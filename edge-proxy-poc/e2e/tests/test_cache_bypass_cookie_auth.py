"""E2E — Test 3: Cache bypass on Cookie / Authorization."""

import pytest
from utils.assertions import assert_status


@pytest.mark.cache
class TestCacheBypass:
    """Requests with Cookie or Authorization must bypass cache."""

    def test_bypass_on_cookie(self, client, purge_all):
        """Request with Cookie header → BYPASS, no caching."""
        r = client.get(
            "/cache/public-long",
            headers={"Cookie": "session_id=abc123"},
        )
        assert_status(r, 200)
        cache = (r.headers.get("X-Edge-Cache") or "").upper()
        assert cache == "BYPASS", f"Expected BYPASS with Cookie, got '{cache}'"

    def test_bypass_on_authorization(self, client, purge_all):
        """Request with Authorization header → BYPASS, no caching."""
        r = client.get(
            "/cache/public-long",
            headers={"Authorization": "Bearer sometoken"},
        )
        assert_status(r, 200)
        cache = (r.headers.get("X-Edge-Cache") or "").upper()
        assert cache == "BYPASS", f"Expected BYPASS with Authorization, got '{cache}'"

    def test_bypass_not_stored(self, client, purge_all):
        """After bypass, a clean request still gets MISS (not HIT from bypass)."""
        # First: bypass
        client.get(
            "/cache/public-long",
            headers={"Cookie": "session_id=xyz"},
        )

        # Second: clean request — should be MISS (nothing was stored from bypass)
        r2 = client.get("/cache/public-long")
        assert_status(r2, 200)
        cache = (r2.headers.get("X-Edge-Cache") or "").upper()
        assert cache in ("MISS", "STORE", "MISS; STORE"), (
            f"Expected MISS after bypass (nothing stored), got '{cache}'"
        )
