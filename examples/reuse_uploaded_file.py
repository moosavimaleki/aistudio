"""استفادهٔ دوباره از یک فایل پیش‌تر آپلودشده در دو درخواست GenerateContent.

Set AISTUDIO_FILE_ID to a file ID already uploaded to the same AI Studio app
folder/profile. Unlike inlineData, fileData does not upload the bytes again.
"""

from __future__ import annotations

import os

from vertex_client import generate_content, print_result, thinking


file_id = os.environ.get("AISTUDIO_FILE_ID")
if not file_id:
    raise SystemExit("Set AISTUDIO_FILE_ID to an existing file ID before running this example.")

# در fileData فقط reference فرستاده می‌شود؛ byte/base64 فایل دوباره ارسال نمی‌شود.
part = {"fileData": {"fileId": file_id, "mimeType": os.environ.get("AISTUDIO_FILE_MIME_TYPE", "text/plain")}}

for prompt in ("این فایل را در یک جمله خلاصه کن.", "سه نکتهٔ کلیدی همین فایل را فهرست کن."):
    response = generate_content(
        "gemini-3.5-flash-lite",
        {
            # همان fileId در هر دو درخواست به‌کار می‌رود.
            "contents": [{"role": "user", "parts": [{"text": prompt}, part]}],
            "generationConfig": {**thinking(), "temperature": 0.1, "maxOutputTokens": 180},
        },
    )
    print(f"\nPrompt: {prompt}")
    print_result(response)
