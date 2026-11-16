# API مستقیم ChatGPT با Go

## استفاده از سرور اصلی پروژه

سه حالت با `CHATGPT_MODE` انتخاب می‌شوند:

| مقدار | رفتار |
|---|---|
| `browser` | مسیر قبلی مرورگر با نمایش پنجره |
| `headless` | مسیر مرورگر با Chrome headless |
| `direct` | ارسال Go و Sentinel محلی، بدون مرورگر ChatGPT |

در دو حالت مرورگر، تنظیم نمایش Chrome در این پروژه مشترک است و برای
مرورگرهای AI Studio هم اعمال می‌شود. مقدار خالی، رفتار قبلی مبتنی بر مدل و
`CHROME_HEADLESS` را حفظ می‌کند. فلگ قدیمی `CHATGPT_DIRECT_NATIVE` فقط زمانی
خوانده می‌شود که `CHATGPT_MODE` تنظیم نشده باشد.

برای اجرای محلی سرور اصلی با نشست موجود:

```bash
CHATGPT_MODE=direct \
CHATGPT_SESSION_FILE="$HOME/.local/state/chatgpt-web/session.json" \
REDIS_URL=redis://127.0.0.1:6379/0 \
go run ./cmd/gencontent
```

Redis همان وابستگی فعلی پروژه برای pool گفتگو و metrics است. در حالت مستقیم،
راه‌اندازی سرور به خواندن کوکی Google یا سلامت سرویس مرورگر وابسته نیست.

برای Docker، در `.env` ریشه قرار بده:

```dotenv
CHATGPT_MODE=direct
```

و بعد `docker compose up -d --build` را اجرا کن. image جدید Node و SDK runner
را دارد؛ کوکی‌ها از mount موجود `CHATGPT_COOKIES` خوانده می‌شوند. مقدار
`CHATGPT_SESSION_FILE` فقط وقتی لازم است که به‌جای این پوشه از snapshot استفاده
کنی؛ در Docker مسیر آن باید داخل کانتینر قابل‌دسترسی باشد.

در هر سه حالت endpoint یکسان است:

```bash
curl http://127.0.0.1:3346/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"chatgpt-web","messages":[{"role":"user","content":"سلام"}]}'
```

در حالت مستقیم می‌توان مدل مشخصی مثل `chatgpt/gpt-6-thinking-standard` هم
فرستاد. نام `chatgpt-web` در این حالت به مدل پیش‌فرض مستقیم نگاشت می‌شود.
پاسخ، شناسهٔ گفتگو و parent را در `lab_metadata` دارد؛ برای ادامه کافی است
`conversation_id` را در JSON بعدی بفرستی. گفتگوهای مبتنی بر تاریخچهٔ کامل
نیز از pool قبلی پروژه استفاده می‌کنند. در `direct` هیچ fallback به مرورگر
انجام نمی‌شود.

برای تست همین endpoint اصلی:

```bash
python3 scripts/smoke_chatgpt_integrated.py \
  --url http://127.0.0.1:3346 \
  --report /tmp/chatgpt-integrated-20.json
```

## فرمان مستقل اختیاری

این فرمان مستقل از TUI، مرورگر، افزونه، Token Factory و daemon.sock اجرا می‌شود.
درخواست‌ها، نشست، خواندن SSE و HTTP server با Go هستند. تنها اجرای SDK
Sentinel با Node 22+ انجام می‌شود؛ فایل SDK runner داخل همین پروژه موجود است.

## اجرا

از ریشهٔ پروژه:

```bash
go run ./cmd/chatgpt-direct -listen 127.0.0.1:8788
```

به‌صورت پیش‌فرض از نشست ذخیره‌شدهٔ نسخهٔ chatgpt-web در
`~/.local/state/chatgpt-web/session.json` استفاده می‌کند. این فایل فقط برای
احراز هویت خوانده می‌شود؛ برنامه به Python یا اجرای پروژهٔ قبلی نیاز ندارد.
می‌توان مسیر را صریح داد:

```bash
go run ./cmd/chatgpt-direct \
  -session /path/to/session.json \
  -sentinel-script assets/sentinel/sentinel-node-probe.js
```

یا از export کوکی Netscape موجود پروژه استفاده کرد:

```bash
go run ./cmd/chatgpt-direct -cookie-dir ./CHATGPT_COOKIES
```

`-session` و `-cookie-dir` جایگزین هم هستند. گزینه‌های `-proxy`، `-model`،
`-upstream` و `-listen` نیز موجودند. `-upstream` برای سرور آزمایش محلی یا
staging قابل تنظیم است. برنامه `.env` را خودکار نمی‌خواند.

## شروع و ادامهٔ گفتگو

```bash
curl http://127.0.0.1:8788/chat \
  -H 'Content-Type: application/json' \
  -d '{"message":"سلام"}'
```

پاسخ شامل `reply`، `conversation_id` و `parent_message_id` است.
برای پیام بعدی فقط شناسهٔ گفتگو را برگردان:

```bash
curl http://127.0.0.1:8788/chat \
  -H 'Content-Type: application/json' \
  -d '{"conversation_id":"RETURNED_ID","message":"خوبی؟"}'
```

شناسهٔ parent از پاسخ قبلی نگه داشته می‌شود؛ بعد از restart از تاریخچهٔ سرور
بازیابی می‌شود. درخواست‌ها به‌ترتیب اجرا می‌شوند. لغو و timeout
از HTTP handler تا شبکه و subprocess منتقل می‌شوند. شکست ارسال باعث ارسال
خودکار مجدد نمی‌شود؛ برای نتیجهٔ مبهم، تاریخچه را پیش از تلاش دوباره بررسی کن.

برای هر ارسال، مدارک Sentinel تازه ساخته می‌شوند. هر دو مدرک proof و VM
در مرحلهٔ finalize قرار می‌گیرند. پاسخ نهایی با وضعیت `finished_successfully`
همراه با پایان SSE (`[DONE]`) تأیید موفقیت است. شناسهٔ پاسخ برای ادامهٔ همان
گفتگو نگه داشته می‌شود تا قبل و بعد از هر پیام، تاریخچه دوباره خوانده نشود.
پس از restart یا برای پاسخ ناقص، تاریخچه خوانده می‌شود. محدودیت `429` این
درخواست خواندنی فقط طبق `Retry-After`، با سقف زمان و تعداد مشخص، قابل تکرار
است؛ درخواست ارسال پیام خودکار تکرار نمی‌شود.

## Docker جدا برای تست مستقیم

این Compose فقط سرویس تست روی `3350` و Redis مستقل آن را می‌سازد؛ سرویس
اصلی روی `3346` را تغییر نمی‌دهد. نشست در `.local/chatgpt-session/session.json`
باید قبلاً با permission مناسب قرار گرفته باشد؛ این مسیر وارد image یا Git
نمی‌شود.

```bash
docker compose -p aistudio-direct-test -f docker-compose.direct-test.yml up -d --build
python3 scripts/smoke_chatgpt_integrated.py --url http://127.0.0.1:3350
```

`DIRECT_TEST_BASE_IMAGE`، `DIRECT_TEST_PROXY` و `DIRECT_TEST_BUILD_PROXY` برای
تنظیم image پایه و proxy آزمایش قابل تغییر هستند. در این محیط، image پشتیبان
محلی `aistudio-api:before-direct-20261116` مبنای ساخت تست است.

## تست

```bash
go test -race ./internal/chatgptdirect ./internal/directapi ./cmd/chatgpt-direct
go vet ./...
python3 scripts/smoke_chatgpt_direct.py \
  --url http://127.0.0.1:8788 \
  --report /tmp/chatgpt-go-direct-20-turns.json
```

تست آخر ۲۰ پیام واقعی می‌فرستد و در پایان اطلاعات پیام اول را می‌پرسد.
`GET /health` فقط سلامت پردازش HTTP را نشان می‌دهد.
