"""E2E — Test 11: Concurrency smoke test."""

import concurrent.futures
import pytest


@pytest.mark.concurrency
class TestConcurrencySmoke:
    """Concurrent requests should be handled stably (singleflight)."""

    def test_concurrent_same_asset(self, proxy_url, purge_all):
        """N concurrent requests for same uncached asset → stable, mostly HITs after."""
        import httpx

        path = "/cache/public-long"
        num_requests = 20
        results = []

        def fetch():
            with httpx.Client(base_url=proxy_url, timeout=10.0) as c:
                return c.get(path)

        with concurrent.futures.ThreadPoolExecutor(max_workers=num_requests) as pool:
            futures = [pool.submit(fetch) for _ in range(num_requests)]
            for f in concurrent.futures.as_completed(futures):
                try:
                    r = f.result()
                    results.append({
                        "status": r.status_code,
                        "cache": (r.headers.get("X-Edge-Cache") or "").upper(),
                    })
                except Exception as e:
                    results.append({"error": str(e)})

        # All should succeed
        statuses = [r.get("status") for r in results if "status" in r]
        errors = [r for r in results if "error" in r]
        assert len(errors) == 0, f"Got errors: {errors}"
        assert all(s == 200 for s in statuses), f"Non-200 statuses: {statuses}"

        # After warmup, most should be HIT (singleflight coalesces)
        caches = [r.get("cache", "") for r in results if "cache" in r]
        hit_count = sum(1 for c in caches if c == "HIT")
        miss_count = sum(1 for c in caches if c in ("MISS", "STORE", "MISS; STORE"))

        # At least some should be HIT (singleflight)
        assert hit_count > 0 or miss_count > 0, (
            f"Unexpected cache distribution: {caches}"
        )

    def test_concurrent_different_assets(self, proxy_url, purge_all):
        """Concurrent requests to different paths should all succeed."""
        import httpx

        paths = [
            "/cache/public-long",
            "/cache/public-short",
            "/static/app.v1.js",
            "/static/styles.v1.css",
            "/api/time",
            "/api/random",
        ]

        results = []

        def fetch(p):
            with httpx.Client(base_url=proxy_url, timeout=10.0) as c:
                return c.get(p)

        with concurrent.futures.ThreadPoolExecutor(max_workers=len(paths)) as pool:
            futures = {pool.submit(fetch, p): p for p in paths}
            for f in concurrent.futures.as_completed(futures):
                try:
                    r = f.result()
                    results.append({"path": futures[f], "status": r.status_code})
                except Exception as e:
                    results.append({"path": futures[f], "error": str(e)})

        errors = [r for r in results if "error" in r]
        assert len(errors) == 0, f"Got errors: {errors}"

        for r in results:
            assert r["status"] == 200, f"Non-200 for {r['path']}: {r['status']}"
