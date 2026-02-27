"""Auth / cookie test endpoints."""

import time
import uuid

from fastapi import APIRouter, Request, Response
from fastapi.responses import JSONResponse

router = APIRouter()

# Simple session store (in-memory, resets on restart)
_sessions: dict[str, dict] = {}


@router.post("/login")
async def login(request: Request):
    """Set a session cookie. Accepts JSON body with optional 'username'."""
    body = {}
    try:
        body = await request.json()
    except Exception:
        pass

    username = body.get("username", "testuser")
    session_id = str(uuid.uuid4())
    _sessions[session_id] = {"username": username, "created": time.time()}

    response = JSONResponse(
        content={"message": "logged in", "username": username},
        headers={"Cache-Control": "no-store"},
    )
    response.set_cookie(
        key="session_id",
        value=session_id,
        httponly=True,
        path="/",
    )
    return response


@router.get("/profile")
async def profile(request: Request):
    """Return profile based on session cookie. Non-cacheable."""
    session_id = request.cookies.get("session_id")
    if not session_id or session_id not in _sessions:
        return JSONResponse(
            status_code=401,
            content={"error": "not authenticated"},
            headers={"Cache-Control": "no-store"},
        )

    session = _sessions[session_id]
    return JSONResponse(
        content={
            "username": session["username"],
            "session_created": session["created"],
            "timestamp": time.time(),
        },
        headers={"Cache-Control": "no-store, private"},
    )
