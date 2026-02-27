"""Debug / observability endpoints."""

import time

from fastapi import APIRouter, Request
from fastapi.responses import JSONResponse

router = APIRouter()


@router.get("/debug/headers")
async def debug_headers(request: Request):
    """Return all received headers — for verifying proxy forwarding."""
    headers_dict = {}
    for key, value in request.headers.items():
        headers_dict[key] = value
    return JSONResponse(
        content={
            "headers": headers_dict,
            "client_host": request.client.host if request.client else None,
            "client_port": request.client.port if request.client else None,
            "timestamp": time.time(),
        },
        headers={"Cache-Control": "no-store"},
    )
