"""WAF test-case endpoints — safe echo endpoints for WAF pattern testing."""

from fastapi import APIRouter, Request
from fastapi.responses import JSONResponse

router = APIRouter()


@router.get("/search")
async def search(request: Request, q: str = ""):
    """Echo search query — used to test XSS/SQLi WAF patterns."""
    return JSONResponse(
        content={
            "query": q,
            "raw_query": str(request.url.query),
            "message": "search results (simulated)",
        },
        headers={"Cache-Control": "no-store"},
    )


@router.post("/submit")
async def submit(request: Request):
    """Echo submitted body — used to test body-inspection WAF rules."""
    content_type = request.headers.get("content-type", "")
    body_bytes = await request.body()
    body_text = body_bytes.decode("utf-8", errors="replace")

    result = {
        "content_type": content_type,
        "body_length": len(body_bytes),
        "body_preview": body_text[:500],
    }

    # Try to parse JSON
    if "json" in content_type:
        try:
            result["parsed_json"] = await request.json()
        except Exception:
            result["parse_error"] = "invalid JSON"

    return JSONResponse(content=result, headers={"Cache-Control": "no-store"})


@router.get("/path-test/{value:path}")
async def path_test(request: Request, value: str):
    """Echo path value — used to test path normalization / encoding."""
    return JSONResponse(
        content={
            "value": value,
            "full_path": str(request.url.path),
            "raw_path": request.scope.get("raw_path", b"").decode(
                "utf-8", errors="replace"
            ),
        },
        headers={"Cache-Control": "no-store"},
    )
