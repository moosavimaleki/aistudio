# ساخت و دانلود ویدیو در AI Studio

این مسیر از `CreateInteractionStream` با مدل `gemini-omni-1.1-flash` استفاده
می‌کند؛ قرارداد آن از HAR ثبت‌شدهٔ سایت گرفته شده است. درخواست شبکه را Go
ارسال می‌کند و توکن تازه از Token Factory موجود دریافت می‌شود.
این مدل در `GET /v1/models` نیز با endpoint مخصوص ویدیو معرفی می‌شود و از مسیر
chat completions قابل استفاده نیست.

## ساخت و دانلود

```bash
BASE_URL="http://127.0.0.1:3346"
curl --fail-with-body -sS --max-time 510 \
  "${BASE_URL%/}/v1/videos/generations" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gemini-omni-1.1-flash",
    "prompt": "یک ویدیوی کوتاه از فیل کارتونی کوچکی بساز که در باغ رنگارنگ شادی می‌کند.",
    "response_format": "url",
    "n": 1
  }' > video-result.json &&
VIDEO_PATH=$(jq -er '.data[0].url' video-result.json) &&
curl --fail-with-body -sS "${BASE_URL%/}${VIDEO_PATH}" -o video.mp4
```

درخواست synchronous است و تا پایان تولید منتظر می‌ماند؛ این endpoint قرارداد
اختصاصی این gateway است، نه API رسمی و asynchronous ویدیو OpenAI. زمان مجاز
هر درخواست هشت دقیقه است. reverse proxy جلوی سرویس نیز باید timeout کافی داشته
باشد. پس از timeout یا پاسخ ناقص، سرویس عملیات تولید را خودکار تکرار نمی‌کند تا
احتمال تولید دوباره و مصرف سهمیه ایجاد نشود.

پاسخ موفق شامل `data[0].url`، `mime_type`، `size_bytes`، `expires_at`، شناسهٔ
interaction در `lab_metadata` و آمار مصرف گزارش‌شدهٔ upstream در `usage` است.
آدرس دانلود نسبی است و روی همین base URL قرار دارد؛ از byte range نیز پشتیبانی
می‌کند. درخواست‌های ناموفق خطای واقعی upstream را با status و body برمی‌گردانند.

فیلد اختیاری `browser_id`، مانند `default` یا `browser2`، پروفایل Google را
انتخاب می‌کند. بدون آن یکی از پروفایل‌های آماده انتخاب می‌شود. دسترسی به مدل و
سهمیهٔ تولید ویدیو به همان حساب وابسته است.

پیش از ساخت، اگر snapshot مرورگر هدر اشتراک نداشته باشد، کلاینت مقدار واقعی
`GetAiStudioBenefitTier` را می‌خواند و در `X-AIStudio-G1-Tier` می‌فرستد.
خطای سهمیه ممکن است داخل stream با HTTP موفق و کد RPC برابر `8` بیاید؛ API آن
را به `429` تبدیل می‌کند و پیام upstream را حفظ می‌کند. در این حالت تولید را
ادامه ندهید؛ این پیام به‌تنهایی تعداد سهمیهٔ روزانه یا زمان reset را تعیین نمی‌کند.

## دریافت مستقیم base64

با `"response_format":"b64_json"`، داده در `data[0].b64_json` می‌آید و روی
سرور ذخیره نمی‌شود. پاسخ ناموفق را قبل از decode بررسی کنید تا فایل خالی ساخته نشود.

```bash
python examples/aistudio_video.py --response-format b64_json --output video.mp4
```

## نمونهٔ پایتون

کتابخانهٔ اضافی لازم نیست. اسکریپت ابتدا تولید را صدا می‌زند، سپس خروجی را
دانلود می‌کند، اندازه و header فایل MP4 را بررسی می‌کند و فقط بعد از موفقیت
فایل مقصد را می‌نویسد:

```bash
python examples/aistudio_video.py \
  --browser-id default \
  --prompt 'Create a short cartoon video of a happy baby elephant with colorful confetti.' \
  --output elephant.mp4
```

برای سرویس دیگر `--base-url` را بدهید یا `AISTUDIO_GENCONTENT_URL` را تنظیم کنید.
مدت، صحنه و سبک را در prompt بنویسید؛ HAR مورد بررسی فیلد مستقلِ مدت، اندازه یا
نسبت تصویر نداشت، بنابراین اجرای دقیق این تنظیمات تضمین نمی‌شود. ورودی تصویر
مرجع و ادامهٔ interaction قبلی در این نسخه پیاده نشده‌اند.

## نگهداری فایل‌ها

حالت `url` فایل‌ها را در `${AISTUDIO_RUNTIME_DIR}/videos` نگه می‌دارد. با
`AISTUDIO_VIDEO_DIR` می‌توان پوشهٔ دیگری تعیین کرد. محدودیت این پوشه برای فایل‌های
این API یک GiB و زمان اعتبار لینک‌ها ۲۴ ساعت است؛ فایل‌های منقضی هنگام ذخیرهٔ
ویدیوی بعدی پاک می‌شوند. با پرشدن محدودیت، پاسخ `507` برمی‌گردد و می‌توان از حالت
`b64_json` استفاده کرد. در تنظیمات پیش‌فرض فعلی این پوشه داخل کانتینر است و با
recreate از دست می‌رود؛ برای ماندگاری باید برای آن volume تعریف کنید.
