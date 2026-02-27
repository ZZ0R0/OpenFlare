"""Shared pytest fixtures for E2E tests."""

import os
import time
import httpx
import pytest

PROXY_URL = os.getenv("PROXY_URL", "http://proxy:8080")
ADMIN_URL = os.getenv("ADMIN_URL", "http://proxy:8081")
ADMIN_TOKEN = os.getenv("ADMIN_TOKEN", "change-me")
ORIGIN_URL = os.getenv("ORIGIN_URL", "http://testsite:8000")


@pytest.fixture(scope="session")
def proxy_url():
    return PROXY_URL


@pytest.fixture(scope="session")
def admin_url():
    return ADMIN_URL


@pytest.fixture(scope="session")
def admin_token():
    return ADMIN_TOKEN


@pytest.fixture(scope="session")
def origin_url():
    return ORIGIN_URL


@pytest.fixture()
def client():
    """HTTP client pointing at the proxy — function-scoped to avoid cookie bleed."""
    with httpx.Client(base_url=PROXY_URL, timeout=10.0) as c:
        yield c


@pytest.fixture(scope="session")
def admin_client():
    """HTTP client pointing at the admin API."""
    with httpx.Client(base_url=ADMIN_URL, timeout=10.0) as c:
        yield c


@pytest.fixture(scope="session")
def origin_client():
    """HTTP client pointing directly at the origin (for reference tests)."""
    with httpx.Client(base_url=ORIGIN_URL, timeout=10.0) as c:
        yield c


@pytest.fixture(autouse=True, scope="session")
def wait_for_services(admin_client, origin_client):
    """Wait for proxy and origin to be healthy before running tests."""
    for name, url, path in [
        ("origin", ORIGIN_URL, "/healthz"),
        ("proxy", PROXY_URL, "/healthz"),
    ]:
        for attempt in range(30):
            try:
                r = httpx.get(url + path, timeout=5.0)
                if r.status_code == 200:
                    break
            except httpx.ConnectError:
                pass
            time.sleep(1)
        else:
            pytest.fail(f"{name} not healthy after 30s")


@pytest.fixture()
def purge_all(admin_client, admin_token):
    """Purge the entire cache before a test."""
    admin_client.post(
        "/admin/cache/purge-all",
        headers={"Authorization": f"Bearer {admin_token}"},
    )
    time.sleep(0.1)  # Small settle time
