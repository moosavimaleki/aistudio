"""orchestration کوتاه flow بارگذاری bytes دریافتی API."""

from __future__ import annotations

from .app_folder import get_app_folder
from .drive import upload_bytes


def upload_content(
    tab,
    content: bytes,
    *,
    mime_type: str,
    name: str | None,
    app_folder_id: str | None,
) -> tuple[str, str]:
    if not content:
        raise ValueError("inlineData content must not be empty")
    folder_id = app_folder_id or get_app_folder(tab)
    file_id = upload_bytes(
        tab,
        name=name or "attachment",
        folder_id=folder_id,
        mime_type=mime_type,
        content=content,
    )
    return file_id, folder_id
