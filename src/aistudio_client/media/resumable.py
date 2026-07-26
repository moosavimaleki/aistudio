"""فایل‌های بزرگ را با protocol رسمی resumable در Drive بارگذاری می‌کند."""

from __future__ import annotations

from ..errors import ClientError, response_error

CHUNK_BYTES = 8 * 1024 * 1024
CHUNK_TIMEOUT_SECONDS = 180


def upload_resumable(
    tab,
    *,
    url: str,
    headers: dict[str, str],
    metadata: dict,
    mime_type: str,
    content: bytes,
):
    initiation = tab.http.request(
        "POST",
        url,
        headers={
            **headers,
            "Content-Type": "application/json; charset=UTF-8",
            "X-Upload-Content-Type": mime_type,
            "X-Upload-Content-Length": str(len(content)),
        },
        json=metadata,
    )
    _apply_response(tab, initiation)
    if not initiation.ok:
        raise response_error(initiation.status_code, initiation.text, phase="UPLOAD")

    upload_url = initiation.headers.get("Location")
    if not upload_url:
        raise ClientError("Drive resumable upload response has no location", phase="UPLOAD")

    for start in range(0, len(content), CHUNK_BYTES):
        end = min(start + CHUNK_BYTES, len(content))
        chunk = content[start:end]
        response = tab.http.request(
            "PUT",
            upload_url,
            headers={
                **headers,
                "Content-Type": mime_type,
                "Content-Length": str(len(chunk)),
                "Content-Range": f"bytes {start}-{end - 1}/{len(content)}",
            },
            data=chunk,
            timeout=CHUNK_TIMEOUT_SECONDS,
            allow_redirects=False,
        )
        _apply_response(tab, response)
        if response.status_code == 308:
            continue
        if not response.ok:
            raise response_error(response.status_code, response.text, phase="UPLOAD")
        return response

    raise ClientError("Drive resumable upload ended without a final response", phase="UPLOAD")


def _apply_response(tab, response) -> None:
    tab.cookies.apply_response(response)
    tab._sync_session()
