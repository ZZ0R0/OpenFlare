"""E2E — Test 8: Cache purge via admin API."""

import pytest
from utils.assertions import assert_status, assert_cache


@pytest.mark.admin
class TestPurge:
    """Admin API cache purge operations."""

    def test_purge_single_key(self, client, admin_client, admin_token, purge_all):
        """Store → purge exact key → next request is MISS."""
        # Store
        r1 = client.get("/cache/public-long")
        assert_status(r1, 200)

        # Verify HIT
        r2 = client.get("/cache/public-long")
        assert_status(r2, 200)
        assert_cache(r2, "HIT")

        # Purge exact key
        pr = admin_client.post(
            "/admin/cache/purge",
            json={"key": "/cache/public-long"},
            headers={"Authorization": f"Bearer {admin_token}"},
        )
        assert pr.status_code in (200, 204), f"Purge failed: {pr.status_code} {pr.text}"

        # Should be MISS now
        r3 = client.get("/cache/public-long")
        assert_status(r3, 200)
        cache3 = (r3.headers.get("X-Edge-Cache") or "").upper()
        assert cache3 in ("MISS", "STORE", "MISS; STORE"), (
            f"Expected MISS after purge, got '{cache3}'"
        )

    def test_purge_all(self, client, admin_client, admin_token, purge_all):
        """Store multiple → purge-all → all MISS."""
        # Store some entries
        client.get("/cache/public-long")
        client.get("/cache/public-short")
        client.get("/static/app.v1.js")

        # Purge all
        pr = admin_client.post(
            "/admin/cache/purge-all",
            headers={"Authorization": f"Bearer {admin_token}"},
        )
        assert pr.status_code in (200, 204)

        # All should be MISS
        for path in ["/cache/public-long", "/cache/public-short", "/static/app.v1.js"]:
            r = client.get(path)
            cache = (r.headers.get("X-Edge-Cache") or "").upper()
            assert "HIT" not in cache, (
                f"Expected MISS after purge-all on {path}, got '{cache}'"
            )

    def test_purge_prefix(self, client, admin_client, admin_token, purge_all):
        """Purge by prefix — all matching entries cleared."""
        # Store entries under /cache/
        client.get("/cache/public-long")
        client.get("/cache/public-short")

        # Purge by prefix
        pr = admin_client.post(
            "/admin/cache/purge-prefix",
            json={"prefix": "/cache/"},
            headers={"Authorization": f"Bearer {admin_token}"},
        )
        assert pr.status_code in (200, 204)

        # Cache entries should be gone
        r = client.get("/cache/public-long")
        cache = (r.headers.get("X-Edge-Cache") or "").upper()
        assert "HIT" not in cache

    def test_purge_requires_auth(self, admin_client):
        """Purge without auth token should be rejected."""
        r = admin_client.post("/admin/cache/purge-all")
        assert r.status_code in (401, 403), (
            f"Expected 401/403 without auth, got {r.status_code}"
        )

    def test_cache_stats(self, client, admin_client, admin_token, purge_all):
        """GET /admin/cache/stats should return stats."""
        # Make some requests to populate cache
        client.get("/cache/public-long")
        client.get("/cache/public-long")

        r = admin_client.get(
            "/admin/cache/stats",
            headers={"Authorization": f"Bearer {admin_token}"},
        )
        assert_status(r, 200)
        data = r.json()
        assert "entries" in data or "hits" in data or "size" in data, (
            f"Stats response missing expected fields: {data}"
        )
