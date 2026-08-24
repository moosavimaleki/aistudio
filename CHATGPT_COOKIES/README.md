# ChatGPT browser cookies

این پوشه فقط برای cookieهای session وب `chatgpt.com` در محیط آزمایشگاهی است.

- فایل باید Netscape `cookies.txt` و دارای پسوند `.txt` باشد.
- هر فایل یک Chrome مستقل می‌سازد و به profileهای Google وابسته نیست.
- cookie را از profile واردشده به ChatGPT با افزونهٔ
  [Get cookies.txt LOCALLY](https://chromewebstore.google.com/detail/get-cookiestxt-locally/cclelndahbckbenkjhflpdbgdldlbecc)
  و قالب Netscape استخراج کنید.
- فایل‌ها را commit نکنید و مقدار cookie یا token را در log قرار ندهید.
- افزونه‌ی کانتینر درخواست ChatGPT را نمی‌سازد؛ صفحه واقعی ChatGPT آن را می‌فرستد.

برای پیدا کردن تمام profileهای محلی Chrome که session سالم ChatGPT دارند، ابتدا
فقط اعتبارسنجی را اجرا کنید:

```bash
python scripts/import_chatgpt_cookies.py --dry-run
```

سپس sessionهای سالم را با قالب Netscape و permission برابر `0640` صادر کنید.
دسترسی group لازم است چون Chrome داخل کانتینر با user جدا ولی group مشترک اجرا می‌شود:

```bash
python scripts/import_chatgpt_cookies.py
```

اسکریپت به‌صورت پیش‌فرض از proxy محلی `http://127.0.0.1:10811` برای بررسی
session استفاده می‌کند. مقدار دیگری را می‌توان با `--proxy` یا
`CHATGPT_PROXY_URL` تعیین کرد. برای به‌روزرسانی فایل‌های ساخته‌شدهٔ قبلی، گزینهٔ
`--replace` را اضافه کنید. مقدار cookie و access token هیچ‌گاه چاپ نمی‌شود.

پس از import، کانتینر را rebuild نکنید؛ یک restart کافی است تا فهرست profileها
دوباره خوانده و cookieها وارد Chrome شوند:

```bash
docker compose restart aistudio
```

اولین فایل `browserId=chatgpt`، فایل دوم `browserId=chatgpt2` و به همین ترتیب است. اگر هیچ فایل `.txt` وجود نداشته باشد، Chrome مخصوص ChatGPT ساخته نمی‌شود.

فایل Netscape فقط bootstrap اولیه است. پس از شروع، خود Chrome session و
cookieهای rotate‌شده را در volume `browser_profiles` نگه می‌دارد و API در وسط
هر request فایل cookie را دوباره اعمال نمی‌کند.

Each Netscape `.txt` file creates one independent ChatGPT Chrome profile. The
file is used only for initial bootstrap; Chrome keeps the live session in the
persistent `browser_profiles` volume. Never commit or publish these files.
