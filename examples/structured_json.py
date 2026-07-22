"""درخواست خروجی JSON با زیرمجموعهٔ پشتیبانی‌شدهٔ responseSchema.

Schema در این مثال همان JSON Schema کامل نیست؛ فقط type، properties، items،
required، enum، description، format و nullable در قرارداد فعلی پشتیبانی می‌شوند.
"""

from vertex_client import generate_content, print_result, thinking


response = generate_content(
    "gemini-3.5-flash",
    {
        "contents": [{"role": "user", "parts": [{"text": "دو کتاب کلاسیک فارسی پیشنهاد بده."}]}],
        "generationConfig": {
            **thinking(2),
            "temperature": 0.1,
            # responseMimeType باید همراه schema فرستاده شود تا مدل JSON برگرداند.
            "responseMimeType": "application/json",
            "responseSchema": {
                "type": "object",
                "properties": {
                    "books": {
                        "type": "array",
                        "items": {
                            "type": "object",
                            "properties": {
                                "title": {"type": "string"},
                                "author": {"type": "string"},
                            },
                            # بدون required، مدل ممکن است یکی از کلیدها را حذف کند.
                            "required": ["title", "author"],
                        },
                    },
                },
                "required": ["books"],
            },
        },
    },
)
print_result(response)
