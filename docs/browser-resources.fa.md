# مصرف منابع مرورگرها

## اجرای headless

در `.env` ریشه مقدار زیر را تنظیم کنید و کانتینر را recreate کنید:

```dotenv
CHROME_HEADLESS=true
```

```bash
docker compose up -d --build aistudio
```

این گزینه `--headless=new` را با viewport برابر ۱۲۸۰×۹۰۰ فعال می‌کند.
افزونه و JavaScript صفحه حفظ می‌شوند؛ Xvfb، Fluxbox، x11vnc و noVNC اجرا
نمی‌شوند. بنابراین در این حالت پنجره‌ای برای اتصال VNC ندارید. برای ورود دستی
یا عیب‌یابی دیداری، مقدار را `false` کنید و دوباره recreate کنید. حالت پیش‌فرض
همچنان دارای پنجره است. کاهش حافظه و زمان پاسخ باید برای تعداد یکسان حساب‌های
آماده و درخواست‌های موفق اندازه‌گیری شود؛ headless به‌تنهایی تضمین سرعت نیست.

تست اختیاری مرورگر واقعی، فقط با صفحه و افزونهٔ مصنوعی:

```bash
CHROME_TEST_EXECUTABLE="$PWD/.local/cloakbrowser/chrome" \
  go test ./internal/chromeprocess -run TestHeadlessExtensionAndInput -v
```

## اندازه‌گیری هر درخواست

از نسخهٔ افزونهٔ `0.3.11`، مسیر مستقیم GPT بعد از probe نه روی New chat می‌زند
و نه صفحه را reload می‌کند. تب و composer موجود برای درخواست بعدی حفظ می‌شوند؛
ساخت payload و conversation واقعی همچنان در Go انجام می‌شود. بازکردن صفحه
فقط هنگام startup یا recovery یک صفحهٔ آماده‌نبودنی انجام می‌شود. توکن هر درخواست
همچنان تازه از پل گرفته می‌شود؛ حذف reload به معنی reuse توکن نیست.

API برای درخواست‌های ثبت‌شده، حتی خطاها، هدرهای `X-Lab-Request-ID`،
`X-Lab-Timings` (JSON برحسب میلی‌ثانیه) و `Server-Timing` می‌دهد. درخواست GPT
موفق همین داده را در `lab_metadata.timings_ms` و Gemini در
`labMetadata.timings_ms` دارد. لاگ JSON با پیشوند `request_timing` و رخداد
`request-timing` داشبورد نیز ثبت می‌شود؛ شناسهٔ مشترک بین gateway و پل مرورگر
برای دنبال‌کردن همان درخواست استفاده می‌شود. متن پیام، cookie و token در این
رخدادها ثبت نمی‌شوند.

مرحله‌های اصلی GPT:

- `conversation_route`: انتخاب/ساخت مکالمه و انتظار pool؛ ممکن است درخواست آغازین هم داشته باشد.
- `gpt_artifacts`: کل آماده‌سازی از پل مرورگر.
- `browser_rotation_queue` و `browser_profile_queue`: انتظار نوبت حساب.
- `browser_page_ready`: بررسی/آماده‌سازی صفحه، شامل cold start احتمالی.
- `browser_extension_dispatch`: انتظار دریافت کار توسط افزونه.
- `browser_composer_submit`: انتظار آماده‌شدن composer و ارسال probe.
- `browser_extension_result`: انتظار باقی‌مانده برای نتیجهٔ افزونه، شامل reset داخل افزونه.
- `browser_cookie_transport`: آماده‌سازی cookie؛ مرحلهٔ قدیمی `browser_page_cleanup` حذف شده است.
- `gpt_turn_prepare`: درخواست prepare مستقیم.
- `gpt_upstream_headers`: ارسال درخواست conversation تا دریافت هدرها؛ شامل اتصال/شبکه و انتظار سرور.
- `gpt_response_body`: دریافت و بازکردن فشرده‌سازی کل پاسخ، شامل ادامهٔ تولید مدل.
- `gpt_parse`: تبدیل پاسخ به نتیجهٔ نهایی.
- `conversation_save`: ثبت نتیجه در pool.

مرحله‌های Gemini: `gemini_profile_select`، `gemini_pool_queue`،
`gemini_bootstrap`، `gemini_upload`، `gemini_token_factory`،
`gemini_upstream_headers` و `gemini_response_body`. زیرمرحله‌های Token Factory
با پیشوند `factory_` می‌آیند. زمان تلاش‌های تکراری جمع می‌شود. مرحله‌های والد و
فرزند هم‌پوشانی دارند؛ جمع همهٔ عددها برابر `total` نیست. هدرها وضعیت زمان
شروع نوشتن پاسخ‌اند؛ لاگ نهایی تا پایان handler را هم دربر می‌گیرد.

```bash
python3 scripts/measure_chatgpt_latency.py --requests 3
```

برای سرور، `--base-url` و متغیر `OPENAI_API_KEY` را تنظیم کنید. این اسکریپت زمان
اولین content غیرخالی، زمان پایان، تعداد chunk و مرحله‌های سرور را گزارش می‌کند.
در مسیر فعلی GPT و Gemini، `stream:true` بافرشده است؛ اولین content پس از
تکمیل پاسخ می‌رسد. `gpt_upstream_headers` زمان اولین توکن مدل نیست. برای TTFT
واقعی، ارسال تدریجی پاسخ باید جداگانه پیاده‌سازی شود.

`CHROME_LOW_RESOURCE_MODE=true` به‌صورت پیش‌فرض فعال است. برای مقایسه با حالت
قبلی، مقدار `false` را در `.env` بگذارید و کانتینر را recreate کنید.

این حالت background networking، component updates، component extensionهای
پس‌زمینه، نصب default apps، Chrome Sync و گزارش Domain Reliability را خاموش
می‌کند. `--renderer-process-limit=2` یک محدودیت نرم برای rendererهای اضافی است؛
site isolation ممکن است به پردازش‌های بیشتری نیاز داشته باشد. تعداد threadهای
raster با `--num-raster-threads=1` کاهش می‌یابد. این فلگ‌ها سقف RAM نیستند.

JavaScript، media، تصویرها، افزونهٔ پل و site isolation فعال می‌مانند. حافظهٔ
V8 محدود نشده است؛ سقف کوچک heap می‌تواند به GC مداوم یا crash منجر شود.
مرورگر همچنان با executable انتخاب‌شده اجرا می‌شود و VNC قابل استفاده است.

بعد از اتصال CDP فقط تب‌های `about:blank` اولیه که تحت کنترل client نیستند
بسته می‌شوند. صفحات حساب‌ها و تب تحت کنترل حفظ می‌شوند. پس از قطع heartbeat،
monitor دست‌کم شش بار با فاصلهٔ پنج ثانیه بررسی می‌کند؛ بین restartهای یک
پروفایل حداقل پنج دقیقه فاصله است. خطای sign-in یا challenge با restart حل
نمی‌شود و monitor آن را خودکار reload نمی‌کند. تمام پروفایل‌ها حفظ می‌شوند.

`CHROME_LAZY_START=true` در Compose پیش‌فرض است: فقط مرورگر پیش‌فرض AI Studio
و ChatGPT در بوت start و warm می‌شوند. دیگر فایل‌های TXT همچنان profile مستقل
دارند و در health و داشبورد با وضعیت `IDLE` دیده می‌شوند. اولین درخواست دارای
`browser_id` آن‌ها را باز و آماده می‌کند؛ Reset داخلی نیز آن‌ها را باز می‌کند.
اگر هیچ ChatGPT آماده‌ای نباشد، انتخاب خودکار می‌تواند یک حساب استفاده‌نشده را
باز کند. اولین درخواست به چنین حسابی هزینهٔ cold start دارد. پس از استفاده،
مرورگر باز می‌ماند؛ این گزینه مرورگرِ دارای کار در حال اجرا را متوقف نمی‌کند.
برای بازکردن همهٔ پروفایل‌ها در بوت، در `.env` مقدار `CHROME_LAZY_START=false`
بگذارید و recreate کنید. این گزینه سقف تعداد مرورگرهای هم‌زمان نیست.

برای مقایسه، بعد از اتمام warm-up در هر دو حالت تعداد یکسانی پروفایل داشته
باشید و میانگین CPU را در یک بازه اندازه بگیرید. RSS پردازش‌ها را جمع نزنید؛
صفحه‌های shared چند بار شمرده می‌شوند. `docker stats` یا cgroup working set
برای مقایسهٔ کل کانتینر مناسب‌تر است. کاهش RAM بدون موفقیت درخواست واقعی
به‌تنهایی نشانهٔ بهبود نیست؛ صفحهٔ لودنشده هم RAM کمی مصرف می‌کند.

منبع فلگ‌ها:
[Google Chrome flags for tooling](https://github.com/GoogleChrome/chrome-launcher/blob/main/docs/chrome-flags-for-tools.md)
و [Chromium content switches](https://chromium.googlesource.com/chromium/src/+/HEAD/content/public/common/content_switches.cc).
