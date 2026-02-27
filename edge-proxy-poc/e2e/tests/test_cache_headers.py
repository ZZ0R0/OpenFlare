"""E2E — Test 2: Cache headers / TTL / origin directives."""

import time
import pytest
from utils.assertions import assert_status, assert_cache


@pytest.mark.cache
class TestCacheHeaders:
    """Respect origin Cache-Control directives."""

    def test_no_store_not_cached(self, client, purge_all):
        """no-store response must not be stored."""
        r1 = client.get("/cache/no-store")
        assert_status(r1, 200)
        cache1 = (r1.headers.get("X-Edge-Cache") or "").upper()
        # Should be MISS or BYPASS — never HIT
        assert "HIT" not in cache1, f"no-store should not produce HIT: {cache1}"

        r2 = client.get("/cache/no-store")
        assert_status(r2, 200)
        cache2 = (r2.headers.get("X-Edge-Cache") or "").upper()
        assert "HIT" not in cache2, f"no-store second request should not be HIT: {cache2}"

    def test_private_not_cached(self, client, purge_all):
        """private response must not be stored in shared cache."""
        r1 = client.get("/cache/private")
        assert_status(r1, 200)

        r2 = client.get("/cache/private")
        assert_status(r2, 200)
        cache2 = (r2.headers.get("X-Edge-Cache") or "").upper()
        assert "HIT" not in cache2, f"private should not produce HIT: {cache2}"

    def test_set_cookie_not_cached(self, client, purge_all):
        """Response with Set-Cookie must not be cached (v1 safety)."""
        r1 = client.get("/cache/with-set-cookie")
        assert_status(r1, 200)

        r2 = client.get("/cache/with-set-cookie")
        assert_status(r2, 200)
        cache2 = (r2.headers.get("X-Edge-Cache") or "").upper()
        assert "HIT" not in cache2, f"Set-Cookie response should not be HIT: {cache2}"

    def test_ttl_expiration(self, client, purge_all):
        """Resource with short TTL: HIT before expiry, MISS after."""
        # /cache/public-short has max-age=5
        r1 = client.get("/cache/public-short")
        assert_status(r1, 200)

        # Should be HIT within TTL
        r2 = client.get("/cache/public-short")
        assert_status(r2, 200)
        assert_cache(r2, "HIT")

        # Wait for TTL to expire
        time.sleep(6)

        r3 = client.get("/cache/public-short")
        assert_status(r3, 200)
        cache3 = (r3.headers.get("X-Edge-Cache") or "").upper()
        assert cache3 in ("MISS", "STORE", "MISS; STORE", "EXPIRED"), (
            f"Expected MISS after TTL expiry, got '{cache3}'"
        )
