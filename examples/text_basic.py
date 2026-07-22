"""ساده‌ترین درخواست متنی به API سازگار با Vertex.

اجرا:
    python examples/text_basic.py
"""

from vertex_client import generate_content, print_result, thinking


response = generate_content(
    "gemini-3.5-flash-lite",
    {
        # هر conversation حداقل یک Content دارد؛ role فقط user یا model است.
        "contents": [{"role": "user", "parts": [{"text": "سلام؛ تهران را در یک جمله معرفی کن."}]}],
        # در قرارداد فعلی آزمایشگاه generationConfig و thinkingConfig اجباری‌اند.
        "generationConfig": thinking(),
    },
)
print_result(response)
