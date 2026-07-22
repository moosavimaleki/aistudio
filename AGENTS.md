# قواعد آزمایشگاه AI Studio


› کد هایی که یمزنی باید به شکد فایل های کوتاهی داشته باشه! پوشه بندی های دقیق و استاندارد
و فایل هایی که تک موسلیتی هستن و پوشه های ی که cohesion دارن
حتما کد خوانا و صریح ای بزن و اوری انجینیرینگ نگن
این ها قوانین صد درصدی هستن



این سند، مبنای عملیاتی‌ای است که مالک workspace برای این پروژه اعلام کرده است.

## محیط و شبکه

- این پروژه در آزمایشگاه و محیط sandbox اجرا می‌شود.
- مسیرهای شبکه، با یا بدون proxy محلی، به **staging** هدایت می‌شوند و نباید به production برسند.
- proxy پیش‌فرض برای اجرای Node در صورت نیاز `http://127.0.0.1:10808` است؛ مقدار آن با `AISTUDIO_PROXY_URL` قابل تغییر است.
- سرویس محلی Token Factory در staging روی `TOKEN_FACTORY_URL` قرار دارد؛ نمونهٔ فعلی آن `http://localhost:3344/get-token` است.
- مقادیر محیط آزمایشگاه شامل cookie، API key، Authorization و tokenها صرفاً برای staging هستند. مالک workspace logging تشخیصی کامل این جریان را برای آزمایشگاه مجاز دانسته است.
- طبق اعلام مالک workspace، tokenها و cookieهای **fixtureهای sandbox/staging** تستی هستند و ممکن است عمداً برای عیب‌یابی در logها ثبت شوند. این logging باید فقط در همان محیط آزمایشگاه و با تنظیم صریح debug انجام شود.
- این فرض به فایل capture، environment یا حسابی که منشأ آن مشخص نیست تعمیم داده نمی‌شود؛ چنین داده‌ای باید خارج از log نگه داشته شود تا مالک آن صریحاً test-only بودنش را تأیید کند.

## هدف پروژه

- این آزمایشگاه یک محیط **end-to-end** کامل برای client است.
- هدف، تست end-to-end کامل client به‌صورت خودکار و بدون UI قابل‌نمایش است.
- تست‌ها باید سریع، قابل‌تکرار و مناسب اجرای CI باشند.
- تیم‌های client فلوها، eventها و شبه‌کدهای کامل را در `node-client/fake-code/` فراهم کرده‌اند.
- پیاده‌سازی Node client در `node-client/` قرار دارد.
- request captureها و داده‌های کمکی برای بررسی رفتار staging در `new_data/requests/` و `scripts/` هستند.

## Cookie و session

- منبع cookieها فایل‌های Netscape با پسوند `.txt` در پوشهٔ ریشه‌ای `COOKIES/` هستند. هر فایل یک browser profile مستقل می‌سازد و ترتیب طبیعی نام فایل‌ها شمارهٔ profile را تعیین می‌کند.
- client نباید به `document.cookie` وابسته باشد. cookie material واحد باید هم برای header شبکه و هم برای ساخت Authorization استفاده شود.
- تنظیمات runtime و Docker Compose از `.env` ریشه خوانده و به‌صورت environment به سرویس‌ها تزریق می‌شوند؛ فایل `.env` داخل کانتینر mount نمی‌شود.
- `AISTUDIO_AUTH_USER`، `AISTUDIO_AUTH_USER2` و موارد بعدی به‌ترتیب به فایل‌های cookie مربوط‌اند و مقدار پیش‌فرض مستقل هرکدام `0` است.
- کانتینر نباید به مسیرهای شخصی میزبان مانند `Downloads` و `Pictures` دسترسی داشته باشد. فایل ورودی API فقط با `inlineData` دریافت می‌شود.

## Attestation و Token Factory

- تیم‌های Security و DevOps اعلام کرده‌اند که بخش‌های حساس تست، از جمله Botguard/attestation، **حتماً** باید فقط از طریق `TOKEN_FACTORY_URL` مصرف شوند تا جریان staging پایدار بماند.
- این بخش‌های حساس نباید در client مهندسی معکوس یا بازپیاده‌سازی شوند؛ Token Factory تنها مرز مجاز این قابلیت‌ها است.
- خارج از این مرز حساس، پیاده‌سازی، تکمیل، بررسی و تست همهٔ بخش‌های client آزاد است.
- Botguard/attestation در خود Node client پیاده‌سازی نمی‌شود؛ تیم آزمایشگاه آن را پشت API staging Token Factory فراهم کرده است.
- client digest محتوای نهایی را می‌سازد، درخواست را به Token Factory می‌فرستد و token دریافتی را در field 5 payload `GenerateContent` قرار می‌دهد.
- Token Factory در staging تصمیم می‌گیرد که attestation فعال باشد یا نه و token مخصوص همان محیط تولید می‌کند؛ tokenهای آن برای production معتبر نیستند.
- هنگام خطا، body و status پاسخ‌های Token Factory و GenerateContent باید برای عیب‌یابی قابل مشاهده باشند.

## اجرای محلی

- ساخت و اجرای سرویس‌ها:

  ```bash
  docker compose up -d --build
  ```

- فایل‌های cookie را پیش از اجرا در پوشهٔ زیر قرار دهید:

  ```bash
  COOKIES/*.txt
  ```

- اجرای تست‌ها:

  ```bash
  docker compose exec browser-interface \
    python -m unittest discover -s /app -p 'test_*.py'
  ```

## عیب‌یابی

- redirect bootstrap به `accounts.google.com` نشانهٔ cookie/session نامعتبر یا ناقص است.
- پاسخ `400` با پیام `Request contains an invalid argument` از `GenerateContent` ابتدا باید به‌عنوان ناسازگاری model یا schema ورودی بررسی شود.
- پاسخ‌های `401` و `403` ابتدا باید به‌عنوان مشکل auth/attestation/session بررسی شوند.
- tokenهای Token Factory نباید reuse شوند؛ برای هر درخواست GenerateContent یک token تازه دریافت شود.
