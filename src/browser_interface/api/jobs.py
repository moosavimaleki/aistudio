"""Extension polling endpoints."""

from typing import Any

from fastapi import APIRouter, Query, Request, Response
from fastapi.responses import JSONResponse

router = APIRouter(prefix="/internal/jobs")


@router.get("/next")
async def next_job(
    request: Request,
    browser_id: str = Query("default", alias="browserId"),
) -> Response:
    job = request.app.state.broker.next(browser_id)
    headers = {"Cache-Control": "no-store"}
    if job is None:
        return Response(status_code=204, headers=headers)
    return JSONResponse(job, headers=headers)


@router.post("/{job_id}/result")
async def complete_job(
    job_id: str,
    body: dict[str, Any],
    request: Request,
    browser_id: str = Query("default", alias="browserId"),
) -> Response:
    if not request.app.state.broker.complete(job_id, body, browser_id):
        return JSONResponse(
            {"error": "job is no longer pending"},
            status_code=404,
        )
    return Response(status_code=204)
