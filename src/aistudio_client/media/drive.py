"""فایل را با OAuth همان tab در پوشهٔ برنامه upload می‌کند."""

from __future__ import annotations

import json
from urllib.parse import quote, urlencode

from ..errors import ClientError, response_error
from .multipart import build_multipart
from .resumable import upload_resumable
from shared import upstream_value

DRIVE_UPLOAD_URL = upstream_value("drive", "upload_url")
AISTUDIO_ORIGIN = upstream_value("aistudio", "origin")
MULTIPART_MAX_BYTES = 5 * 1024 * 1024


def upload_bytes(tab, *, name: str, folder_id: str, mime_type: str, content: bytes) -> str:
    if not tab.oauth_access_token or not tab.runtime:
        raise ClientError("Tab has no OAuth access token", phase="UPLOAD")
    headers = {
        **tab.transport_profile,
        "Authorization": f"Bearer {tab.oauth_access_token}",
        "Origin": tab.auth.origin,
        "Referer": f"{AISTUDIO_ORIGIN}/",
        "X-Goog-AuthUser": tab.runtime.auth_user,
        "X-ClientDetails": _client_details(tab.transport_profile),
        "X-Goog-Encode-Response-If-Executable": "base64",
        "X-JavaScript-User-Agent": "google-api-javascript-client/1.1.0",
        "X-Requested-With": "XMLHttpRequest",
    }
    metadata = {"name": name, "parents": [folder_id]}
    if len(content) > MULTIPART_MAX_BYTES:
        url = f"{DRIVE_UPLOAD_URL}?uploadType=resumable&key={quote(tab.runtime.api_key)}"
        response = upload_resumable(
            tab,
            url=url,
            headers=headers,
            metadata=metadata,
            mime_type=mime_type,
            content=content,
        )
    else:
        boundary, body = build_multipart(name, folder_id, mime_type, content)
        url = f"{DRIVE_UPLOAD_URL}?uploadType=multipart&key={quote(tab.runtime.api_key)}"
        response = tab.http.request(
            "POST",
            url,
            headers={
                **headers,
                "Content-Type": f'multipart/related; boundary="{boundary}"',
            },
            data=body,
        )
        tab.cookies.apply_response(response)
        tab._sync_session()
    if not response.ok:
        raise response_error(response.status_code, response.text, phase="UPLOAD")
    try:
        file_id = response.json()["id"]
    except (json.JSONDecodeError, KeyError, TypeError) as error:
        raise ClientError("Drive upload response has no file id", phase="UPLOAD") from error
    if not isinstance(file_id, str) or not file_id:
        raise ClientError("Drive upload returned an invalid file id", phase="UPLOAD")
    return file_id


def _client_details(profile: dict[str, str]) -> str:
    user_agent = profile.get("User-Agent", "")
    app_version = user_agent.removeprefix("Mozilla/")
    platform = profile.get("sec-ch-ua-platform", '"Linux"').strip('"') + " x86_64"
    return urlencode({
        "appVersion": app_version,
        "platform": platform,
        "userAgent": user_agent,
    })
