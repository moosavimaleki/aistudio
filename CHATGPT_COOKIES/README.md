# ChatGPT browser cookies

این پوشه فقط برای cookieهای session وب `chatgpt.com` در محیط آزمایشگاهی است.

- فایل باید Netscape `cookies.txt` و دارای پسوند `.txt` باشد.
- هر فایل یک Chrome مستقل می‌سازد و به profileهای Google وابسته نیست.
- cookie را از profile واردشده به ChatGPT و با یک ابزار محلی قابل اعتماد استخراج کنید.
- فایل‌ها را commit نکنید و مقدار cookie یا token را در log قرار ندهید.
- افزونه‌ی کانتینر درخواست ChatGPT را نمی‌سازد؛ صفحه واقعی ChatGPT آن را می‌فرستد.

اولین فایل `browserId=chatgpt`، فایل دوم `browserId=chatgpt2` و به همین ترتیب است. اگر هیچ فایل `.txt` وجود نداشته باشد، Chrome مخصوص ChatGPT ساخته نمی‌شود.
