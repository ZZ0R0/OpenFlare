"""Shared HTTP helpers for E2E tests."""

import httpx

DEFAULT_TIMEOUT = 10.0


def get(client: httpx.Client, path: str, **kwargs) -> httpx.Response:
    """GET with sensible defaults."""
    kwargs.setdefault("timeout", DEFAULT_TIMEOUT)
    return client.get(path, **kwargs)


def post(client: httpx.Client, path: str, **kwargs) -> httpx.Response:
    """POST with sensible defaults."""
    kwargs.setdefault("timeout", DEFAULT_TIMEOUT)
    return client.post(path, **kwargs)


def get_cache_header(response: httpx.Response) -> str | None:
    """Return the X-Edge-Cache header value (MISS, HIT, BYPASS, etc.)."""
    return response.headers.get("X-Edge-Cache") or response.headers.get("x-edge-cache")


def get_request_id(response: httpx.Response) -> str | None:
    """Return the X-Request-ID header."""
    return response.headers.get("X-Request-ID") or response.headers.get("x-request-id")
