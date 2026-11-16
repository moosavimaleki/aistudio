# API مستقیم ChatGPT با Go

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

شناسهٔ parent از تاریخچهٔ سرور گرفته می‌شود؛ پس ادامهٔ گفتگو بعد از restart
هم به حافظهٔ محلی وابسته نیست. درخواست‌ها به‌ترتیب اجرا می‌شوند. لغو و timeout
از HTTP handler تا شبکه و subprocess منتقل می‌شوند. شکست ارسال باعث ارسال
خودکار مجدد نمی‌شود؛ برای نتیجهٔ مبهم، تاریخچه را پیش از تلاش دوباره بررسی کن.

برای هر ارسال، مدارک Sentinel تازه ساخته می‌شوند. هر دو مدرک proof و VM
در مرحلهٔ finalize قرار می‌گیرند. پیش از برگرداندن موفقیت، ثبت شناسهٔ دقیق
پیام کاربر و پاسخ کامل در تاریخچهٔ سرور تأیید می‌شود.

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
