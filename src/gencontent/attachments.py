"""inlineData شبکه را پیش از ساخت payload به Drive file id تبدیل می‌کند."""

from __future__ import annotations

import base64
import binascii
from copy import deepcopy


def resolve_inline_data(contents: list[dict], tab) -> list[dict]:
    resolved = deepcopy(contents)
    for content in resolved:
        for part in content.get("parts", []):
            inline = part.get("inlineData") if isinstance(part, dict) else None
            if not inline:
                continue
            file_id = tab.upload_bytes(
                _decode(inline["data"]),
                mime_type=inline["mimeType"],
                name=inline.get("displayName"),
            )
            part.clear()
            part["fileData"] = {"fileId": file_id, "mimeType": inline["mimeType"]}
    return resolved


def _decode(value: str) -> bytes:
    try:
        # google-genai برای bytes از Base64 URL-safe استفاده می‌کند. altchars
        # هر دو الفبای استاندارد و URL-safe را با validation سخت‌گیرانه می‌پذیرد.
        return base64.b64decode(value, altchars=b"-_", validate=True)
    except (ValueError, binascii.Error) as error:
        raise ValueError("inlineData.data must be valid base64") from error
