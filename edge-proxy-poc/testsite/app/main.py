"""
OpenFlare Test Site — FastAPI origin server for E2E proxy testing.

Provides deterministic endpoints for cache, WAF, rate-limit, auth,
robustness, and debug testing.
"""

import logging
import os
import time

from fastapi import FastAPI, Request, HTTPException
from fastapi.responses import HTMLResponse
from fastapi.staticfiles import StaticFiles

from .cache_cases import router as cache_router
from .auth_cases import router as auth_router
from .waf_cases import router as waf_router
from .api_cases import router as api_router
from .robustness_cases import router as robustness_router
from .debug_cases import router as debug_router

# ---------------------------------------------------------------------------
# Logging
# ---------------------------------------------------------------------------
logging.basicConfig(
    level=os.getenv("LOG_LEVEL", "INFO").upper(),
    format="%(asctime)s %(levelname)s %(name)s %(message)s",
)
logger = logging.getLogger("testsite")

# ---------------------------------------------------------------------------
# App
# ---------------------------------------------------------------------------
app = FastAPI(title="OpenFlare TestSite", version="1.0.0")

# Static files
STATIC_DIR = os.path.join(os.path.dirname(__file__), "static")
app.mount("/static", StaticFiles(directory=STATIC_DIR), name="static")

# Routers
app.include_router(cache_router, prefix="/cache", tags=["cache"])
app.include_router(auth_router, prefix="/auth", tags=["auth"])
app.include_router(waf_router, tags=["waf"])
app.include_router(api_router, prefix="/api", tags=["api"])
app.include_router(robustness_router, tags=["robustness"])
app.include_router(debug_router, tags=["debug"])

# ---------------------------------------------------------------------------
# Root / health / private
# ---------------------------------------------------------------------------

@app.get("/")
def index():
    """Simple cacheable HTML page."""
    html = """<!DOCTYPE html>
<html><head><title>OpenFlare TestSite</title></head>
<body><h1>Welcome to OpenFlare TestSite</h1>
<p>This site serves as a test origin for the edge proxy.</p>
</body></html>"""
    return HTMLResponse(content=html, headers={
        "Cache-Control": "public, max-age=30",
    })


@app.get("/healthz")
def healthz():
    return {"status": "ok"}


@app.get("/readyz")
def readyz():
    return {"status": "ready"}


@app.get("/private")
async def private_endpoint(request: Request):
    """Requires Authorization header or session cookie."""
    auth = request.headers.get("Authorization")
    session = request.cookies.get("session_id")
    if not auth and not session:
        raise HTTPException(status_code=401, detail="Unauthorized")
    return {
        "message": "private content",
        "user": auth or session,
        "timestamp": time.time(),
    }
