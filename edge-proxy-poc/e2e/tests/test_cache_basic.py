"""E2E — Test 1: Cache basic (MISS then HIT on static assets)."""

import pytest
from utils.assertions import assert_status, assert_cache


@pytest.mark.cache
class TestCacheBasic:
    """First GET → MISS/STORE, second GET → HIT, identical content."""

    def test_static_js_miss_then_hit(self, client, purge_all):
        # First request — expect MISS
        r1 = client.get("/static/app.v1.js")
        assert_status(r1, 200)
        cache1 = (r1.headers.get("X-Edge-Cache") or "").upper()
        assert cache1 in ("MISS", "STORE", "MISS; STORE"), (
            f"Expected MISS or STORE on first request, got '{cache1}'"
        )

        # Second request — expect HIT
        r2 = client.get("/static/app.v1.js")
        assert_status(r2, 200)
        assert_cache(r2, "HIT")

        # Content must be identical
        assert r1.text == r2.text, "Cached content differs from original"

    def test_static_css_miss_then_hit(self, client, purge_all):
        r1 = client.get("/static/styles.v1.css")
        assert_status(r1, 200)

        r2 = client.get("/static/styles.v1.css")
        assert_status(r2, 200)
        assert_cache(r2, "HIT")
        assert r1.text == r2.text

    def test_static_image_miss_then_hit(self, client, purge_all):
        r1 = client.get("/static/image.png")
        assert_status(r1, 200)

        r2 = client.get("/static/image.png")
        assert_status(r2, 200)
        assert_cache(r2, "HIT")
        assert r1.content == r2.content

    def test_cache_public_short(self, client, purge_all):
        """GET /cache/public-short — cacheable with short TTL."""
        r1 = client.get("/cache/public-short")
        assert_status(r1, 200)

        r2 = client.get("/cache/public-short")
        assert_status(r2, 200)
        assert_cache(r2, "HIT")

    def test_x_edge_cache_header_present(self, client, purge_all):
        """Every proxied response must have X-Edge-Cache header."""
        r = client.get("/cache/public-long")
        assert "X-Edge-Cache" in r.headers or "x-edge-cache" in r.headers, (
            f"Missing X-Edge-Cache header. Headers: {dict(r.headers)}"
        )
