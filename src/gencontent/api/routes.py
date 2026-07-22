"""HTTP routes and stable error responses."""

import asyncio

from fastapi import APIRouter, Request
from fastapi.responses import HTMLResponse, JSONResponse

from aistudio_client.errors import ClientError

from gencontent.pool import PoolOverError
from gencontent.dashboard import render_dashboard
from .adapters import legacy_input, vertex_input, vertex_response
from .models import GenerateContentBody, VertexGenerateContentBody


router = APIRouter()


@router.get("/", response_class=HTMLResponse)
async def dashboard(request: Request):
    service = request.app.state.service
    pool, profiles = await asyncio.gather(
        asyncio.to_thread(request.app.state.pool.snapshot),
        asyncio.to_thread(service.profiles.all),
    )
    return HTMLResponse(render_dashboard(pool, profiles))


@router.get("/health")
async def health(request: Request):
    stats = await asyncio.to_thread(request.app.state.pool.stats)
    profiles = await asyncio.to_thread(request.app.state.service.profiles.all)
    return {
        "ok": True,
        "service": "gencontent",
        "pool": stats,
        "profiles": [profile.__dict__ for profile in profiles],
    }


@router.post("/generate-content")
async def generate_content(body: GenerateContentBody, request: Request):
    service = request.app.state.service
    model = body.model or service.settings.model
    if not model:
        return JSONResponse(status_code=400, content={"error": "model is required"})
    outcome = await asyncio.to_thread(
        service.generate,
        legacy_input(model, body),
    )
    return {
        "state": "READY",
        "tabId": outcome.tab_id,
        "browserId": outcome.browser_id,
        "authUser": outcome.auth_user,
        "tabGenerateCount": outcome.generate_count,
        "text": outcome.result.final_text,
        "chunkCount": len(outcome.result.chunks),
        "chunks": outcome.result.chunks,
        "finishReason": outcome.result.finish_reason,
        "usage": outcome.result.usage,
        "conversationMetadata": outcome.result.conversation_metadata,
    }


@router.post(
    "/v1/projects/{project}/locations/{location}/publishers/google/"
    "models/{model}:generateContent"
)
async def vertex_generate_content(
    project: str,
    location: str,
    model: str,
    body: VertexGenerateContentBody,
    request: Request,
):
    outcome = await asyncio.to_thread(
        request.app.state.service.generate,
        vertex_input(model, body),
    )
    response = vertex_response(model, outcome)
    response["labMetadata"].update({"project": project, "location": location})
    return response


async def pool_over_handler(_request: Request, error: PoolOverError):
    return JSONResponse(
        status_code=503,
        content={"error": "pool over", "code": "TAB_POOL_OVER", "message": str(error)},
    )


async def client_error_handler(_request: Request, error: ClientError):
    status = error.status if error.status and 400 <= error.status < 600 else 502
    return JSONResponse(
        status_code=status,
        content={
            "error": str(error),
            "phase": error.phase,
            "status": error.status,
            "responseBody": error.response_body,
            "diagnostics": error.diagnostics,
        },
    )


async def value_error_handler(_request: Request, error: ValueError):
    return JSONResponse(status_code=422, content={"error": str(error)})
