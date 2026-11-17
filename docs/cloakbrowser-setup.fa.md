# نصب مرورگر الزامی CloakBrowser

سرویس دیگر به Chrome معمولی برنمی‌گردد. پیش از `docker compose up`، پوشهٔ
باینری Linux x86-64 CloakBrowser را در `.local/cloakbrowser/` قرار دهید؛ فایل
`.local/cloakbrowser/chrome` باید اجرایی باشد. این پوشه در Git ثبت نمی‌شود و
به‌صورت read-only در کانتینر mount می‌شود. نسخهٔ آزمایش‌شده در این پروژه
`146.0.7680.177.5` است. از باینری رسمی و سازگار با مجوز خودتان استفاده کنید.

برای کپی از نسخه‌ای که از قبل روی همین ماشین نصب شده است:

```bash
mkdir -p .local/cloakbrowser
rsync -a qwen-gate/.qwen/cloakbrowser-cache/chromium-146.0.7680.177.5/ .local/cloakbrowser/
docker compose up -d --build
```

پروفایل‌های CloakBrowser در volume جدا نگهداری می‌شوند؛ پروفایل‌های قدیمی
Chrome حذف یا بازنویسی نمی‌شوند. cookieها همچنان از `COOKIES/` و
`CHATGPT_COOKIES/` خوانده می‌شوند. برنامه در نبود باینری با خطای صریح متوقف
می‌شود. هیچ کلیکی روی صفحهٔ چالش خودکار نمی‌شود.

برای مقایسهٔ مستقل بارگذاری صفحه با دو پروفایل موقت، بدون دست‌زدن به سرویس:

```bash
SMOKE_PROXY_URL=http://127.0.0.1:10808 bash scripts/compare_browser_load.sh \
  "$PWD/.local/cloakbrowser/chrome"
```

این مقایسه فقط عنوان صفحه را می‌سنجد و جای تست API با نشست واقعی را نمی‌گیرد.

روی سرور کم‌حافظه، فقط یک پروفایل ChatGPT را هم‌زمان راه‌اندازی کنید:

```bash
docker compose -f docker-compose.yml -f docker-compose.server-cloak.yml up -d --build
```

این فایل فقط `chatgpt3` را فعال می‌کند؛ Gemini همچنان فعال است. برای تغییر
پروفایل، مقدار `CHATGPT_ENABLED_BROWSER_IDS` را در overlay ویرایش کنید و همان
دستور را دوباره اجرا کنید. چالش صفحهٔ ChatGPT روی IP سرور ممکن است نیاز به
تکمیل دستی از طریق VNC داشته باشد؛ سرویس آن را خودکار حل نمی‌کند.
