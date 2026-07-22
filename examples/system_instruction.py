"""استفاده از system instruction و پارامترهای sampling.

System instruction خارج از history مکالمه قرار می‌گیرد و برای تعیین رفتار کلی
مدل مناسب است؛ متن واقعی درخواست همچنان باید در ``contents`` باشد.
"""

from vertex_client import generate_content, print_result, thinking


response = generate_content(
    "gemini-3.5-flash",
    {
        "systemInstruction": {
            # هر instruction نیز مانند Content از parts تشکیل می‌شود.
            "parts": [{"text": "تو یک ویراستار فارسی هستی. پاسخ‌ها را حداکثر در سه bullet کوتاه بده."}],
        },
        "contents": [{"role": "user", "parts": [{"text": "این جمله را بهتر کن: من دیروز رفتم بازار و چیز خریدم."}]}],
        "generationConfig": {
            # دمای کمتر معمولاً پاسخ‌های یکنواخت‌تر و قابل‌تکرارتری می‌دهد.
            **thinking(2),
            "temperature": 0.2,
            "topP": 0.8,
            "topK": 20,
            "maxOutputTokens": 160,
        },
    },
)
print_result(response)
