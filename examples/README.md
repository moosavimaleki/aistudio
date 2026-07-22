# مثال‌های Python برای API سازگار با Vertex

این مثال‌ها فقط با HTTP و کتابخانهٔ استاندارد Python نوشته شده‌اند؛ به SDK گوگل یا دسترسی مستقیم به MakerSuite نیاز ندارند. مقصد پیش‌فرض سرویس محلی `http://127.0.0.1:3346` است.

ابتدا سرویس آزمایشگاه را بالا بیاورید:

```bash
cd /home/h-mousavi/Projects/Hamed/aistudio-api
docker compose up -d
curl http://127.0.0.1:3346/health
```

سپس هر مثال را از ریشهٔ repository اجرا کنید:

```bash
python examples/text_basic.py
python examples/chat_history.py
python examples/system_instruction.py
python examples/structured_json.py
python examples/one_file.py
python examples/multiple_files.py
```

## SDK رسمی `google-genai`

مثال [google_genai_sdk.py](google_genai_sdk.py) همان endpoint را با SDK رسمی
فراخوانی می‌کند:

```bash
python -m pip install google-genai
python examples/google_genai_sdk.py
```

SDK در حالت Vertex پیش از ارسال درخواست credential می‌خواهد. مثال یک
`LocalLabCredentials` حداقلی دارد که **فقط** برای gateway محلی staging معتبر
است؛ این credential Google نیست و نباید به هیچ محیط دیگری منتقل شود.

در قرارداد فعلی، `thinkingBudget` با SDK کار می‌کند. `thinkingLevel` رسمی SDK
مانند `LOW` هنوز به enum عددی داخلی آزمایشگاه mapping نشده است.

برای تغییر آدرس سرویس یا بخش‌های مسیر Vertex:

```bash
AISTUDIO_GENCONTENT_URL=http://127.0.0.1:3346 \
AISTUDIO_VERTEX_PROJECT=lab \
AISTUDIO_VERTEX_LOCATION=us-central1 \
python examples/text_basic.py
```

## فایل جدید در برابر فایل موجود

`one_file.py` و `multiple_files.py` از `inlineData` استفاده می‌کنند. سرویس برای هر `inlineData`، byteها را در Drive application folder همان profile آپلود می‌کند و سپس شناسهٔ آن را داخل درخواست MakerSuite می‌گذارد. بنابراین اجرای دوبارهٔ این مثال‌ها upload دوباره انجام می‌دهد.

`reuse_uploaded_file.py` از `fileData.fileId` استفاده می‌کند و upload انجام نمی‌دهد. شناسه باید قبلاً در همان AI Studio profile ساخته شده باشد:

```bash
AISTUDIO_FILE_ID='existing-drive-file-id' \
AISTUDIO_FILE_MIME_TYPE='text/plain' \
python examples/reuse_uploaded_file.py
```

پاسخ فعلی Vertex-compatible API شناسهٔ فایل‌هایی را که از `inlineData` آپلود شده‌اند برنمی‌گرداند؛ بنابراین یک client عمومی نمی‌تواند همان ID تازه را از پاسخ اول بردارد. مثال reuse عمداً این ID را ورودی می‌گیرد تا رفتار واقعی API شفاف بماند.

## محدودیت‌های قرارداد فعلی

- `generationConfig.thinkingConfig` اجباری است؛ همهٔ مثال‌ها آن را دارند.
- `candidateCount` در wire format فعلی mapping تأییدشده ندارد و رد می‌شود.
- `fileData.fileId` فقط در همان application folder/profile معتبری که فایل در آن ایجاد شده استفاده شود.
- مدل در URL به‌صورت `gemini-…` نوشته می‌شود؛ service آن را به `models/gemini-…` تبدیل می‌کند.
