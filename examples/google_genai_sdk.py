"""استفاده از SDK رسمی ``google-genai`` با gateway محلی Vertex-compatible.

نصب وابستگی فقط برای اجرای این مثال:

    python -m pip install google-genai
    python examples/google_genai_sdk.py

این مثال مخصوص آزمایشگاه است. ``LocalLabCredentials`` فقط SDK را قادر می‌کند
درخواست HTTP را به gateway محلی بفرستد؛ هیچ اعتبارنامهٔ واقعی Google نیست و
هرگز نباید در production استفاده شود.
"""

from __future__ import annotations

import mimetypes
import os
from pathlib import Path

from google import genai
from google.auth.credentials import Credentials
from google.genai import types


class LocalLabCredentials(Credentials):
    """Credential حداقلی برای transport محلی SDK.

    SDK رسمی Vertex قبل از ارسال درخواست یک credential می‌خواهد. gateway محلی
    این header را validate نمی‌کند، چون احراز هویت واقعی درون AI Studio flow
    آزمایشگاه انجام می‌شود. بنابراین این کلاس فقط برای همین endpoint local است.
    """

    def __init__(self) -> None:
        super().__init__()
        self.token = "local-lab-token"

    @property
    def expired(self) -> bool:
        return False

    @property
    def valid(self) -> bool:
        return True

    def refresh(self, request) -> None:
        # token ثابت local نیازی به refresh ندارد.
        return None


base_url = os.environ.get("AISTUDIO_GENCONTENT_URL", "http://127.0.0.1:3346")
project = os.environ.get("AISTUDIO_VERTEX_PROJECT", "lab")
location = os.environ.get("AISTUDIO_VERTEX_LOCATION", "us-central1")

# vertexai=True باعث می‌شود SDK مسیر استاندارد Vertex را بسازد:
# /v1/projects/{project}/locations/{location}/publishers/google/models/...:generateContent
client = genai.Client(
    vertexai=True,
    project=project,
    location=location,
    credentials=LocalLabCredentials(),
    http_options=types.HttpOptions(base_url=base_url, api_version="v1"),
)
# gemini-3.1-pro-preview
# gemini-3.5-flash-lite
response = client.models.generate_content(
    model="gemini-3.1-pro-preview",
    contents="سلام؛ سمنان را در یک جمله معرفی کن.",
    config=types.GenerateContentConfig(
        temperature=0.2,
        max_output_tokens=100,
        # API فعلی آزمایشگاه thinkingBudget را با SDK رسمی می‌پذیرد.
        # thinkingLevel استاندارد (LOW/MEDIUM/...) هنوز به levelEnum داخلی map نشده است.
        thinking_config=types.ThinkingConfig(thinking_budget=64),
    ),
)

print(response.text)

# همان قرارداد با متد stream رسمی SDK نیز قابل فراخوانی است. upstream
# آزمایشگاه فعلاً پاسخ را یک‌جا جمع می‌کند، بنابراین یک chunk نهایی می‌رسد.
stream = client.models.generate_content_stream(
    model="gemini-3.1-pro-preview",
    contents="فقط بنویس: جریان سازگار است",
    config=types.GenerateContentConfig(
        temperature=0,
        max_output_tokens=30,
        thinking_config=types.ThinkingConfig(thinking_budget=32),
    ),
)
print("".join(chunk.text or "" for chunk in stream))

# فایل با API خود SDK به inlineData تبدیل می‌شود. gateway آن را در application
# folder همان Chrome profile آپلود می‌کند و شناسهٔ حاصل را به GenerateContent می‌دهد.
default_file = Path(__file__).parent / "assets" / "meeting-notes.txt"
file_path = Path(os.environ.get("AISTUDIO_GENAI_FILE", default_file))
mime_type = mimetypes.guess_type(file_path.name)[0] or "application/octet-stream"

file_response = client.models.generate_content(
    model="gemini-3.1-pro-preview",
    contents=[
        types.Content(
            role="user",
            parts=[
                types.Part(
                    text=os.environ.get(
                        "AISTUDIO_GENAI_FILE_PROMPT",
                        "نام فایل و موضوع کلی آن را خیلی کوتاه بگو.",
                    ),
                ),
                types.Part.from_bytes(
                    data=file_path.read_bytes(),
                    mime_type=mime_type,
                ),
            ],
        ),
    ],
    config=types.GenerateContentConfig(
        temperature=0,
        max_output_tokens=100,
        thinking_config=types.ThinkingConfig(thinking_budget=32),
    ),
)
print(file_response.text)
