"""Robustness / upstream failure test endpoints."""

import asyncio
import time

from fastapi import APIRouter, Request, UploadFile, File
from fastapi.responses import JSONResponse, Response

router = APIRouter()


@router.get("/slow")
async def slow_response(delay_ms: int = 1000):
    """Respond after a configurable delay (ms). Used to test upstream timeouts."""
    delay_s = min(delay_ms, 30000) / 1000.0  # Cap at 30s
    await asyncio.sleep(delay_s)
    return JSONResponse(
        content={
            "delayed_ms": delay_ms,
            "actual_delay_ms": int(delay_s * 1000),
            "timestamp": time.time(),
        },
        headers={"Cache-Control": "no-store"},
    )


@router.get("/error/{code}")
def error_endpoint(code: int):
    """Return a specific HTTP status code."""
    if code < 100 or code > 599:
        code = 500
    body = {"error": True, "status_code": code, "message": f"Simulated {code}"}
    return JSONResponse(content=body, status_code=code)


@router.post("/upload")
async def upload_endpoint(request: Request):
    """Accept an upload and report its size. Used to test body size limits."""
    body = await request.body()
    return JSONResponse(
        content={
            "received_bytes": len(body),
            "content_type": request.headers.get("content-type", ""),
            "timestamp": time.time(),
        },
        headers={"Cache-Control": "no-store"},
    )


@router.post("/webhooks/test")
async def webhook_test(request: Request):
    """Non-cacheable webhook endpoint — idempotence not guaranteed."""
    body = await request.body()
    return JSONResponse(
        content={
            "received": True,
            "body_length": len(body),
            "timestamp": time.time(),
        },
        status_code=202,
        headers={"Cache-Control": "no-store"},
    )
