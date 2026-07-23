# Observability و Dashboard آزمایشگاه

این subsystem آمار عملیاتی GenContent، Tab Pool، Chrome profile، cookie session،
Token Factory و فایل‌ها را بدون ذخیرهٔ prompt، cookie، Authorization یا token
جمع‌آوری می‌کند.

## مسیرها

- Dashboard: `http://127.0.0.1:3346/`
- دادهٔ Dashboard: `GET /dashboard/data?window=60`
- Health ساده: `GET /health`

Dashboard هر پنج ثانیه refresh می‌شود و بازه‌های ۱۵ دقیقه، یک ساعت و ۲۴ ساعت
دارد.

## جداسازی از منطق برنامه

جمع‌آوری آمار در این مرزها انجام می‌شود:

- `MetricsMiddleware`: status و latency درخواست HTTP
- `ObservedGenerateService`: نتیجه و latency GenerateContent
- `ObservedTabPool`: acquire، wait، release و discard
- `BrowserEventMetrics`: تبدیل eventهای موجود browser-interface به metric
- `SessionDiagnostics`: metadata امن cookie/session

هیچ‌کدام از این اجزا payload، token یا جریان business را تغییر نمی‌دهند.
نوشتن metric به‌صورت fail-open است؛ خطای observability درخواست اصلی را fail
نمی‌کند.

## ذخیره‌سازی Redis

داده‌های زمانی در Hashهای یک‌دقیقه‌ای ذخیره می‌شوند:

```text
lab:metrics:v1:minute:{epoch-minute}
```

هر Hash به‌صورت پیش‌فرض پس از ۴۸ ساعت حذف می‌شود. eventهای اخیر داخل یک List
محدود قرار دارند و به‌صورت پیش‌فرض هفت روز retention دارند. registry مربوط به
Tab حداکثر به‌اندازهٔ ظرفیت pool رشد می‌کند.

برای هر request یک Redis key ساخته نمی‌شود. درخواست‌های جاری فقط عضو یک Sorted
Set مشترک هستند و عضوهای منقضی هنگام خواندن پاک می‌شوند.

## تنظیمات

همهٔ تنظیمات اختیاری‌اند:

```dotenv
METRICS_NAMESPACE=lab:metrics:v1
METRICS_RETENTION_SECONDS=172800
METRICS_EVENT_RETENTION_SECONDS=604800
METRICS_EVENT_LIMIT=200
```

برای کاهش حافظه می‌توان retention دادهٔ دقیقه‌ای را مثلاً به ۲۴ ساعت کاهش داد:

```dotenv
METRICS_RETENTION_SECONDS=86400
```

## داده‌های Cookie

فقط این metadataها نمایش داده می‌شوند:

- تعداد cookie
- تعداد cookieهای خانوادهٔ SAPISID
- revision عددی
- نزدیک‌ترین expiry قابل مشاهده
- زمان آخرین sync و ready
- معتبر بودن revision فایل منبع
- redirect یا warm-up error

مقدار cookie، fingerprint، digest، نام کامل فایل، Authorization و token در
Redis یا Dashboard ذخیره نمی‌شوند.

## معنای Latency Percentile

برای کنترل حافظه، latency خام هر درخواست ذخیره نمی‌شود. درخواست‌ها داخل
bucketهای ثابت قرار می‌گیرند و `P50` و `P95` تقریبی از همان histogram محاسبه
می‌شوند:

```text
100ms, 250ms, 500ms, 1s, 2.5s, 5s, 10s, 30s, 60s
```

بنابراین نمایش `P95 = 10s` یعنی percentile موردنظر در bucket ده‌ثانیه قرار
گرفته است، نه اینکه مقدار دقیق آن حتماً ده ثانیه باشد.
