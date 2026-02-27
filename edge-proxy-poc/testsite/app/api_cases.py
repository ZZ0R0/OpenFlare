"""API dynamic endpoints — non-cacheable by design."""

import random
import string
import time

from fastapi import APIRouter, Request
from fastapi.responses import JSONResponse

router = APIRouter()


@router.get("/time")
def api_time():
    """JSON with current timestamp — non-cacheable."""
    return JSONResponse(
        content={"timestamp": time.time(), "iso": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())},
        headers={"Cache-Control": "no-store"},
    )


@router.get("/random")
def api_random():
    """JSON with random data — non-cacheable."""
    return JSONResponse(
        content={
            "random_int": random.randint(0, 1_000_000),
            "random_string": "".join(random.choices(string.ascii_letters, k=16)),
            "timestamp": time.time(),
        },
        headers={"Cache-Control": "no-store"},
    )


@router.get("/echo")
async def api_echo(request: Request):
    """Echo query params and useful headers for debugging."""
    return JSONResponse(
        content={
            "method": request.method,
            "path": str(request.url.path),
            "query_params": dict(request.query_params),
            "headers": {
                "host": request.headers.get("host"),
                "x-forwarded-for": request.headers.get("x-forwarded-for"),
                "x-forwarded-proto": request.headers.get("x-forwarded-proto"),
                "x-request-id": request.headers.get("x-request-id"),
                "user-agent": request.headers.get("user-agent"),
                "accept": request.headers.get("accept"),
                "accept-encoding": request.headers.get("accept-encoding"),
            },
            "timestamp": time.time(),
        },
        headers={"Cache-Control": "no-store"},
    )
