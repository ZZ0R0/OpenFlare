"""Polling / retry helpers for E2E tests."""

import time
from typing import Callable, TypeVar

T = TypeVar("T")


def poll_until(
    fn: Callable[[], T],
    predicate: Callable[[T], bool],
    timeout: float = 10.0,
    interval: float = 0.5,
    description: str = "condition",
) -> T:
    """Poll fn() until predicate(result) is True or timeout."""
    deadline = time.monotonic() + timeout
    last_result = None
    while time.monotonic() < deadline:
        last_result = fn()
        if predicate(last_result):
            return last_result
        time.sleep(interval)
    raise TimeoutError(
        f"Timed out waiting for {description} after {timeout}s. "
        f"Last result: {last_result}"
    )
