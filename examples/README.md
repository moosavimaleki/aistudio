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
python examples/chatgpt_openai.py
python examples/chatgpt_200_questions.py
python examples/chatgpt_image.py
```

`chatgpt_openai.py` دو turn متوالی OpenAI-compatible را اجرا می‌کند. حالت
پیش‌فرض `chatgpt/gpt-5.6-pro` است: درخواست نهایی را Go می‌فرستد و Chrome فقط
artifactهای محافظت‌شدهٔ تازه را آماده می‌کند. مثال IDهای conversation و parent
پاسخ اول را به turn دوم می‌دهد؛ gateway هیچ state محلی برای چت نگه نمی‌دارد.
برای تست مسیر کاملاً UI-based از دستور زیر استفاده کنید:

```bash
CHATGPT_MODEL=chatgpt-web python examples/chatgpt_openai.py
```

هر دو حالت به `CHATGPT_COOKIES/*.txt` نیاز دارند. مقصد با
`OPENAI_BASE_URL`، profile با `CHATGPT_BROWSER_ID` و timeout با
`OPENAI_TIMEOUT` قابل تغییر است.

`chatgpt_200_questions.py` به‌صورت پیش‌فرض ۲۰۰ سؤال را ذیل یک conversation
می‌فرستد و میان هر دو سؤال یک ثانیه صبر می‌کند. با `Ctrl+C` متوقف می‌شود.
برای smoke کوتاه‌تر یا تغییر فاصله:

```bash
CHATGPT_QUESTION_COUNT=3 CHATGPT_DELAY_SECONDS=1 \
python examples/chatgpt_200_questions.py
```

`chatgpt_image.py` تصویر تولیدشده توسط خود صفحهٔ ChatGPT را از API
`/v1/images/generations` دریافت و در `chatgpt-image.jpg` ذخیره می‌کند. فرمت
واقعی تصویر را خود UI ChatGPT تعیین می‌کند؛ هنگام تعیین مسیر دلخواه، پسوند را
با فرمت خروجی هماهنگ کنید. برای تغییر prompt و مسیر خروجی:

```bash
CHATGPT_IMAGE_PROMPT='A small watercolor fox reading a book' \
CHATGPT_IMAGE_OUTPUT=/tmp/fox.jpg \
python examples/chatgpt_image.py
```

## SDK رسمی `google-genai`

مثال [google_genai_sdk.py](google_genai_sdk.py) همان endpoint را با SDK رسمی
برای متن، stream و فایل فراخوانی می‌کند:

```bash
python -m pip install google-genai
python examples/google_genai_sdk.py
```

فایل پیش‌فرض `examples/assets/meeting-notes.txt` است. برای ارسال فایل دیگری:

```bash
AISTUDIO_GENAI_FILE=/path/to/audio.ogg \
AISTUDIO_GENAI_FILE_PROMPT='متن کامل فایل را استخراج کن' \
python examples/google_genai_sdk.py
```

مدل مثال نیز قابل تغییر است:

```bash
AISTUDIO_MODEL=gemini-3.5-flash-lite python examples/google_genai_sdk.py
```

SDK در حالت Vertex پیش از ارسال درخواست credential می‌خواهد. مثال یک
`LocalLabCredentials` حداقلی دارد که **فقط** برای gateway محلی staging معتبر
است؛ این credential Google نیست و نباید به هیچ محیط دیگری منتقل شود.

هر دو حالت `thinkingBudget` و `thinkingLevel` رسمی SDK پشتیبانی می‌شوند، اما
نباید هم‌زمان ارسال شوند. `candidateCount` نیز به field واقعی upstream منتقل
می‌شود؛ مدل `gemini-3.5-flash-lite` فعلاً فقط مقدار `1` را قبول می‌کند.
خروجی stream فعلاً buffered است؛ یعنی transport رسمی SSE است اما پاسخ upstream
پس از کامل‌شدن generation در یک chunk تحویل داده می‌شود.

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
- `candidateCount` پشتیبانی می‌شود، ولی محدودیت تعداد candidate به مدل وابسته است.
- `functionCall`، `functionResponse`، `executableCode`،
  `codeExecutionResult` و `videoMetadata` به fieldهای MakerSuite تبدیل می‌شوند.
- FunctionResponse در مدل‌های Gemini 3 باید thought signature پاسخ قبلی مدل را
  همراه history حفظ کند؛ signature ساختگی معتبر نیست.
- `fileData.fileId` فقط در همان application folder/profile معتبری که فایل در آن ایجاد شده استفاده شود.
- مدل در URL به‌صورت `gemini-…` نوشته می‌شود؛ service آن را به `models/gemini-…` تبدیل می‌کند.
