"""ارسال تاریخچهٔ چند-turn به یک مدل دیگر.

API هیچ state مکالمه‌ای را از متن درخواست حدس نمی‌زند؛ برای ادامهٔ گفتگو باید
turnهای قبلی را به‌ترتیب و با role درست داخل ``contents`` بفرستید.
"""

from vertex_client import generate_content, print_result, thinking


response = generate_content(
    "gemini-3.1-pro-preview",
    {
        "contents": [
            # turn نخست کاربر
            {"role": "user", "parts": [{"text": "نام سه شهر تاریخی ایران را بگو."}]},
            # پاسخ قبلی مدل که client آن را نگه داشته است
            {"role": "model", "parts": [{"text": "اصفهان، یزد و شیراز."}]},
            # پرسش follow-up کاربر
            {"role": "user", "parts": [{"text": "برای هرکدام یک دلیل کوتاه بنویس."}]},
        ],
        # level و پارامترهای sampling برای هر درخواست قابل تنظیم‌اند.
        "generationConfig": {**thinking(3), "temperature": 0.35, "maxOutputTokens": 300},
    },
)
print_result(response)
