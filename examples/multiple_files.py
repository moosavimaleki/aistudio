"""ارسال چند فایل محلی در یک درخواست.

Both inlineData parts are independently uploaded by the service before
GenerateContent. Use reuse_uploaded_file.py when a Drive fileId already exists.
"""

from pathlib import Path

from vertex_client import generate_content, inline_data, print_result, thinking


assets = Path(__file__).with_name("assets")
response = generate_content(
    "gemini-3.5-flash",
    {
        "contents": [{
            "role": "user",
            "parts": [
                # یک instruction مشترک برای تمام فایل‌های همان turn
                {"text": "یادداشت و جدول را با هم بررسی کن و اختلاف مبلغ را بگو."},
                # هر inlineData یک upload مستقل در service ایجاد می‌کند.
                {"inlineData": inline_data(assets / "meeting-notes.txt", mime_type="text/plain")},
                {"inlineData": inline_data(assets / "budget.csv", mime_type="text/csv")},
            ],
        }],
        "generationConfig": {**thinking(3), "temperature": 0.15, "maxOutputTokens": 300},
    },
)
print_result(response)
