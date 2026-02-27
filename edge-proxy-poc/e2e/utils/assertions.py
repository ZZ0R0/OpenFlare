"""Assertion helpers for E2E tests."""

import httpx


def assert_status(response: httpx.Response, expected: int, msg: str = ""):
    """Assert response status code with helpful diagnostics."""
    actual = response.status_code
    if actual != expected:
        detail = (
            f"Expected status {expected}, got {actual}. "
            f"URL: {response.url} "
            f"Headers: {dict(response.headers)} "
            f"Body preview: {response.text[:200]}"
        )
        if msg:
            detail = f"{msg} — {detail}"
        raise AssertionError(detail)


def assert_cache(response: httpx.Response, expected: str, msg: str = ""):
    """Assert X-Edge-Cache header value."""
    actual = (
        response.headers.get("X-Edge-Cache")
        or response.headers.get("x-edge-cache")
        or ""
    ).upper()
    expected_upper = expected.upper()
    if actual != expected_upper:
        detail = (
            f"Expected X-Edge-Cache={expected_upper}, got '{actual}'. "
            f"URL: {response.url}"
        )
        if msg:
            detail = f"{msg} — {detail}"
        raise AssertionError(detail)


def assert_has_header(response: httpx.Response, header: str, msg: str = ""):
    """Assert response has a specific header."""
    lower_headers = {k.lower(): v for k, v in response.headers.items()}
    if header.lower() not in lower_headers:
        detail = f"Expected header '{header}' not found. Headers: {dict(response.headers)}"
        if msg:
            detail = f"{msg} — {detail}"
        raise AssertionError(detail)
