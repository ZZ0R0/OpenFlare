"""Cache behavior test endpoints."""

import hashlib
import time
from datetime import datetime, timezone

from fastapi import APIRouter, Request, Response

router = APIRouter()

# Fixed content for deterministic testing
ETAG_CONTENT = "This is ETag-controlled content for cache testing."
ETAG_VALUE = '"' + hashlib.md5(ETAG_CONTENT.encode()).hexdigest() + '"'
LAST_MODIFIED_TIME = datetime(2025, 1, 1, 0, 0, 0, tzinfo=timezone.utc)
LAST_MODIFIED_STR = LAST_MODIFIED_TIME.strftime("%a, %d %b %Y %H:%M:%S GMT")


@router.get("/public-short")
def public_short():
    """Cache-Control: public, max-age=5"""
    return Response(
        content='{"data": "public-short", "ts": ' + str(time.time()) + "}",
        media_type="application/json",
        headers={"Cache-Control": "public, max-age=5"},
    )


@router.get("/public-long")
def public_long():
    """Cache-Control: public, max-age=60"""
    return Response(
        content='{"data": "public-long", "ts": ' + str(time.time()) + "}",
        media_type="application/json",
        headers={"Cache-Control": "public, max-age=60"},
    )


@router.get("/no-store")
def no_store():
    """Cache-Control: no-store"""
    return Response(
        content='{"data": "no-store", "ts": ' + str(time.time()) + "}",
        media_type="application/json",
        headers={"Cache-Control": "no-store"},
    )


@router.get("/private")
def private_cache():
    """Cache-Control: private"""
    return Response(
        content='{"data": "private", "ts": ' + str(time.time()) + "}",
        media_type="application/json",
        headers={"Cache-Control": "private"},
    )


@router.get("/with-set-cookie")
def with_set_cookie():
    """Response with Set-Cookie header — should NOT be cached."""
    return Response(
        content='{"data": "with-set-cookie", "ts": ' + str(time.time()) + "}",
        media_type="application/json",
        headers={
            "Cache-Control": "public, max-age=60",
            "Set-Cookie": "tracking=abc123; Path=/",
        },
    )


@router.get("/etag")
def etag_endpoint(request: Request):
    """ETag / If-None-Match support."""
    if_none_match = request.headers.get("If-None-Match")
    if if_none_match == ETAG_VALUE:
        return Response(status_code=304, headers={"ETag": ETAG_VALUE})
    return Response(
        content=ETAG_CONTENT,
        media_type="text/plain",
        headers={
            "ETag": ETAG_VALUE,
            "Cache-Control": "public, max-age=60",
        },
    )


@router.get("/last-modified")
def last_modified_endpoint(request: Request):
    """Last-Modified / If-Modified-Since support."""
    ims = request.headers.get("If-Modified-Since")
    if ims:
        # Simple comparison: if IMS matches or is after our date, return 304
        try:
            ims_dt = datetime.strptime(ims, "%a, %d %b %Y %H:%M:%S GMT").replace(
                tzinfo=timezone.utc
            )
            if ims_dt >= LAST_MODIFIED_TIME:
                return Response(
                    status_code=304,
                    headers={"Last-Modified": LAST_MODIFIED_STR},
                )
        except ValueError:
            pass  # Malformed date, serve full response

    return Response(
        content="This is Last-Modified controlled content.",
        media_type="text/plain",
        headers={
            "Last-Modified": LAST_MODIFIED_STR,
            "Cache-Control": "public, max-age=60",
        },
    )
