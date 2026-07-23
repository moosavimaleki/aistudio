"""ارسال یک فایل محلی با Vertex ``inlineData``.

سرویس آزمایشگاه base64 را decode می‌کند، فایل را در application folder همان
profile آپلود می‌کند، سپس به‌جای inlineData یک fileData داخلی برای MakerSuite می‌سازد.
"""

from pathlib import Path

from vertex_client import generate_content, inline_data, print_result, thinking


file_path = Path(__file__).with_name("assets") / "meeting-notes.txt"
response = generate_content(
    "gemini-3.5-flash-lite",
    {
        "contents": [{
            "role": "user",
            "parts": [
                # ترتیب partها مهم است و متن می‌تواند کنار فایل قرار بگیرد.
                {"text": "متن کامل این فایل رو استخراج کن"},
                # اجرای دوبارهٔ این مثال، upload تازه انجام می‌دهد.
                # {"inlineData": inline_data("/home/h-mousavi/Downloads/s.mp3")},
                {"inlineData": inline_data("/home/h-mousavi/Downloads/l.mp3")},
            ],
        }],
        "generationConfig": {**thinking(), "temperature": 0.2},
    },
    # آپلود resumable و پردازش فایل صوتی بزرگ ممکن است چند دقیقه طول بکشد.
    timeout=600,
)
print_result(response)
