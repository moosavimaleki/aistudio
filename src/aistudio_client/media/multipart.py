"""بدنهٔ multipart/related سازگار با Drive upload را می‌سازد."""

from __future__ import annotations

import base64
import json
from uuid import uuid4


def build_multipart(name: str, folder_id: str, mime_type: str, content: bytes) -> tuple[str, bytes]:
    boundary = uuid4().hex
    metadata = json.dumps(
        {"name": name, "parents": [folder_id]},
        ensure_ascii=False,
        separators=(",", ":"),
    )
    body = (
        f"--{boundary}\r\n"
        "Content-Type: application/json; charset=UTF-8\r\n\r\n"
        f"{metadata}\r\n"
        f"--{boundary}\r\n"
        f"Content-Type: {mime_type}\r\n"
        "Content-Transfer-Encoding: base64\r\n\r\n"
        f"{base64.b64encode(content).decode('ascii')}\r\n"
        f"--{boundary}--\r\n"
    ).encode("utf-8")
    return boundary, body
