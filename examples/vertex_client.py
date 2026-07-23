"""کلاینت کوچک و آموزشی برای endpoint سازگار با Vertex در آزمایشگاه.

این فایل عمداً فقط از کتابخانهٔ استاندارد Python استفاده می‌کند تا مثال‌ها
بدون SDK خارجی قابل خواندن و اجرا باشند. هر مثال دیگر تنها payload خود را
می‌سازد و ارسال HTTP را به این فایل می‌سپارد.
"""

from __future__ import annotations

import base64
import json
import mimetypes
import os
from pathlib import Path
from typing import Any
from urllib.error import HTTPError
from urllib.parse import quote
from urllib.request import Request, urlopen


DEFAULT_BASE_URL = "http://127.0.0.1:3346"
DEFAULT_PROJECT = "lab"
DEFAULT_LOCATION = "us-central1"


def generate_content(
    model: str,
    body: dict[str, Any],
    *,
    timeout: float = 120.0,
) -> dict[str, Any]:
    """درخواست GenerateContent را به gateway محلی Vertex-compatible می‌فرستد.

    ``model`` می‌تواند با یا بدون پیشوند ``models/`` باشد. برخلاف Vertex
    production، project و location اینجا فقط بخشی از URL آزمایشگاه‌اند.
    """
    response = _request("POST", _vertex_url(model), body, timeout)
    return response


def inline_data(path: str | Path, *, mime_type: str | None = None) -> dict[str, str]:
    """یک part از نوع Vertex ``inlineData`` برای فایل محلی می‌سازد.

    The server uploads this content to its application Drive folder before it
    calls MakerSuite. Every request containing inlineData performs an upload.
    """
    file_path = Path(path)
    content = file_path.read_bytes()
    if not content:
        raise ValueError(f"File is empty: {file_path}")
    detected_type = mimetypes.guess_type(file_path.name)[0]
    if mime_type and detected_type and mime_type != detected_type:
        raise ValueError(
            f"mime_type {mime_type!r} does not match {file_path.name!r}; "
            f"use {detected_type!r} or omit mime_type"
        )
    return {
        "mimeType": mime_type or detected_type or "application/octet-stream",
        "displayName": file_path.name,
        "data": base64.b64encode(content).decode("ascii"),
    }


def text(response: dict[str, Any]) -> str:
    """تمام متن نخستین candidate را از پاسخ Vertex استخراج می‌کند.

    پاسخ stream ممکن است متن را در چند part برگرداند؛ ترتیب partها حفظ می‌شود
    و partهای غیرمتنی، مانند thoughtSignature، در خروجی متن وارد نمی‌شوند.
    """
    try:
        parts = response["candidates"][0]["content"]["parts"]
    except (KeyError, IndexError, TypeError) as error:
        raise ValueError(f"Response has no text candidate: {json.dumps(response, ensure_ascii=False)}") from error

    texts = [
        part["text"]
        for part in parts
        if isinstance(part, dict) and isinstance(part.get("text"), str)
    ]
    if not texts:
        raise ValueError(f"Response has no text candidate: {json.dumps(response, ensure_ascii=False)}")
    return "".join(texts)


def print_result(response: dict[str, Any]) -> None:
    # labMetadata بخشی از پاسخ کمکی آزمایشگاه است و در Vertex production وجود ندارد.
    print(text(response))
    print("\n--- lab metadata ---")
    print(json.dumps(response.get("labMetadata", {}), ensure_ascii=False, indent=2))


def thinking(level: int = 4) -> dict[str, Any]:
    """حداقل generationConfig لازم برای قرارداد فعلی آزمایشگاه را برمی‌گرداند.

    سطح thinking یک enum عددیِ تأییدشده در wire format فعلی است. اگر مدل یا
    قرارداد تغییر کرد، این عدد را از روی capture جدید به‌روز کنید.
    """
    return {"thinkingConfig": {"levelEnum": level}}


def _vertex_url(model: str) -> str:
    # endpoint با الگوی رسمی Vertex ساخته می‌شود، اما به سرویس محلی می‌رود.
    base_url = os.environ.get("AISTUDIO_GENCONTENT_URL", DEFAULT_BASE_URL).rstrip("/")
    model_name = model.removeprefix("models/")
    project = os.environ.get("AISTUDIO_VERTEX_PROJECT", DEFAULT_PROJECT)
    location = os.environ.get("AISTUDIO_VERTEX_LOCATION", DEFAULT_LOCATION)
    return (
        f"{base_url}/v1/projects/{quote(project, safe='')}/locations/{quote(location, safe='')}"
        f"/publishers/google/models/{quote(model_name, safe='')}:generateContent"
    )


def _request(method: str, url: str, body: dict[str, Any], timeout: float) -> dict[str, Any]:
    # ensure_ascii=False باعث می‌شود متن فارسی در wire به‌صورت UTF-8 بماند.
    request = Request(
        url,
        data=json.dumps(body, ensure_ascii=False).encode("utf-8"),
        method=method,
        headers={"Content-Type": "application/json"},
    )
    try:
        with urlopen(request, timeout=timeout) as response:
            return json.loads(response.read())
    except HTTPError as error:
        detail = error.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"GenerateContent returned HTTP {error.code}: {detail}") from error
