"""Public health, bootstrap and token endpoints."""

from typing import Any

from fastapi import APIRouter, Request
from fastapi.responses import JSONResponse

from ..errors import BrowserIdentityMismatch, InvalidCookieSession, UnknownBrowser
from ..events import emit

router = APIRouter()


@router.get("/health")
async def health(request: Request) -> JSONResponse:
    browsers = request.app.state.browsers.status()
    return JSONResponse(
        {
            "backend": "container-extension",
            "connected": bool(browsers) and all(item["connected"] for item in browsers),
            "pendingJobs": sum(item["pendingJobs"] for item in browsers),
            "browsers": browsers,
        },
        headers={"Cache-Control": "no-store"},
    )


@router.post("/bootstrap")
async def bootstrap(body: dict[str, Any], request: Request) -> JSONResponse:
    try:
        browser_id = request.app.state.browsers.resolve(body.get("browserId"))
    except Exception as error:
        return _failure("bootstrap-error", error)
    cookies = body.get("cookies") or request.app.state.browsers.configured_cookies(browser_id)
    if not isinstance(cookies, str) or not cookies.strip():
        return JSONResponse({"error": "cookies are required"}, status_code=400)
    try:
        auth_user = str(
            body.get("authUser", request.app.state.browsers.auth_user(browser_id))
        )
        request.app.state.browsers.assert_identity(browser_id, cookies, auth_user)
        result = await request.app.state.browsers.session(browser_id).prepare(
            cookies,
            auth_user,
        )
        result["browserId"] = browser_id
        return JSONResponse(result)
    except Exception as error:
        return _failure("bootstrap-error", error)


@router.post("/get-token")
async def get_token(body: dict[str, Any], request: Request) -> JSONResponse:
    try:
        result = await request.app.state.tokens.create(body)
        return JSONResponse(result)
    except Exception as error:
        return _failure("token-error", error)


def _failure(event: str, error: Exception) -> JSONResponse:
    emit(event, message=str(error), code=type(error).__name__)
    if isinstance(error, InvalidCookieSession):
        status = 401
    elif isinstance(error, (UnknownBrowser, BrowserIdentityMismatch)):
        status = 400
    else:
        status = 500
    return JSONResponse({"error": str(error)}, status_code=status)
