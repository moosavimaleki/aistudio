# کلاینت ChatGPT Web: فلو، مرزها و Smoke Test

این سند معماری کلاینت session-based وب ChatGPT را توضیح می‌دهد. این قابلیت فقط در آزمایشگاه و روی مسیر staging اجرا می‌شود.

## اصل طراحی

Go یک کلاینت HTTP تقلیدی برای endpoint خصوصی ChatGPT نیست. Go فقط API عمومی آزمایشگاه، صف job، profileها، timeout و تبدیل پاسخ را مدیریت می‌کند. درخواست واقعی ChatGPT را صفحه‌ی واقعی `chatgpt.com` در Chrome می‌سازد.

هیچ‌کدام از موارد زیر در پروژه پیاده‌سازی نمی‌شوند:

- ساخت یا بازسازی Sentinel، Proof-of-Work، Turnstile یا Arkose
- TLS یا browser fingerprint impersonation
- استخراج access token و headerهای محافظتی از HAR یا Chrome
- ارسال مستقیم `/backend-api/f/conversation` از Go
- reuse کردن tokenهای متعلق به درخواست دیگر

## شواهد HAR

capture فعلی این endpointهای اصلی را نشان می‌دهد:

| مرحله | endpoint صفحه | مالک مرحله در معماری ما |
|---|---|---|
| آماده‌سازی گفتگو | `POST /backend-api/f/conversation/prepare` | خود صفحه ChatGPT |
| آماده‌سازی الزامات | `POST /backend-api/sentinel/chat-requirements/prepare` | خود صفحه ChatGPT |
| نهایی‌سازی الزامات | `POST /backend-api/sentinel/chat-requirements/finalize` | خود صفحه ChatGPT |
| درخواست Sentinel | `POST /backend-api/sentinel/req` | خود صفحه ChatGPT |
| تولید پاسخ واردشده | `POST /backend-api/f/conversation` | خود صفحه ChatGPT |
| تولید پاسخ anonymous | `POST /backend-anon/f/conversation` | خود صفحه ChatGPT |
| فهرست گفتگوها | `GET /backend-api/conversations` | خود صفحه ChatGPT |

پاسخ conversation از نوع `text/event-stream` است. capture فعلی eventهای `delta_encoding` و `delta` دارد. متن پاسخ در patchهای `p/o/v` و عمدتاً در مسیر `/message/content/parts/0` با عمل `append` می‌آید.

## تفاوت با gpt4free

provider بررسی‌شده در gpt4free این کارها را انجام می‌دهد:

1. یک HTTP session با impersonation کروم می‌سازد.
2. `conversation/prepare` را مستقیم صدا می‌زند.
3. نسخه‌ی قدیمی `sentinel/chat-requirements` را مستقیم صدا می‌زند.
4. در صورت نیاز Proof-of-Work، Turnstile یا Arkose را تولید یا استخراج می‌کند.
5. headerهای محافظتی را روی درخواست مستقیم conversation قرار می‌دهد.
6. SSE را خارج از مرورگر parse می‌کند.

مراحل ۱ تا ۵ در این پروژه ممنوع و حذف شده‌اند. فقط ایده‌ی عمومی parsing stream با قرارداد واقعی HAR مقایسه شده است. ضمن اینکه HAR جدید نشان می‌دهد endpoint قدیمی chat-requirements به فلو `prepare/finalize` تغییر کرده و کپی provider قدیمی شکننده خواهد بود.

## الگوریتم مجاز

```text
OpenAI-compatible request
        |
        v
Go gateway: validate + render messages
        |
        v
browser-interface job (kind=chatgpt.generate)
        |
        v
Chrome extension service worker
        |
        v
content script: arm capture + fill composer + click Send
        |
        v
real ChatGPT page: prepare + Sentinel + conversation
        |
        v
MAIN-world observer: clone only the conversation response
        |
        v
SSE parser -> text -> job result -> OpenAI response
```

MAIN-world observer فقط `window.fetch` را مشاهده می‌کند. ورودی، header و response اصلی را تغییر نمی‌دهد. از response یک clone خوانده می‌شود تا UI همان پاسخ طبیعی را دریافت کند. اگر stream به‌دلیل handoff یا retry داخلی صفحه متن نهایی نداشته باشد، content script متن آخرین پیام assistant را از DOM واقعی صفحه می‌خواند؛ status شبکه همچنان از همان response صفحه گزارش می‌شود.

هر profile در نسخه‌ی اول فقط یک job هم‌زمان دارد. پیش از job، تب ChatGPT به صفحه‌ی شروع برمی‌گردد تا درخواست stateless باشد. model واقعی توسط UI انتخاب می‌شود؛ API فعلاً نتیجه را با نام `chatgpt-web` گزارش می‌کند و model خصوصی در payload ساخته نمی‌شود.

## Cookie و session

fleet مرورگرها مستقل است. هر فایل `COOKIES/*.txt` یک Chrome مخصوص AI Studio و هر فایل `CHATGPT_COOKIES/*.txt` یک Chrome مخصوص ChatGPT می‌سازد. مثلاً دو فایل Google و یک فایل ChatGPT در مجموع سه process مرورگر ایجاد می‌کنند.

پیش از job، cookieها داخل Chrome مستقل ChatGPT اعمال می‌شوند. پس از پاسخ موفق، cookieهای چرخیده‌ی `chatgpt.com` به همان فایل برمی‌گردند.

## قرارداد API آزمایشگاه

درخواست اولیه:

```http
POST /v1/chat/completions
Content-Type: application/json

{
  "model": "chatgpt-web",
  "messages": [
    {"role": "user", "content": "Reply with exactly: OK"}
  ]
}
```

نسخه‌ی اول text-only است. `stream=true` نیز پذیرفته می‌شود، اما تا زمانی که bridge streaming داخلی اضافه شود پاسخ به‌صورت یک chunk با header `X-Lab-Streaming-Mode: buffered` برمی‌گردد.

## Smoke Test مرحله‌ای

Smoke معتبر باید این gateها را به‌ترتیب اثبات کند:

1. `POST /v1/chat/completions` در gateway پذیرفته شود.
2. job با `kind=chatgpt.generate` به profile درست dispatch شود.
3. content script روی `chatgpt.com` پیام job را دریافت کند.
4. composer آماده باشد و کلیک Send رخ دهد.
5. observer درخواست دقیق `/backend-api/f/conversation` را ببیند.
6. status پاسخ صفحه `200` و content type آن SSE باشد.
7. حداقل یک patch متن parse شود.
8. API محلی پاسخ `200` سازگار با OpenAI برگرداند.

trace مجاز فقط شامل `jobId`، `browserId`، `kind`، phase، HTTP status و مدت زمان است. prompt، cookie، token، Authorization و headerهای Sentinel نباید log شوند.

## نتیجه Smoke اولیه

Smoke اولیه در gate دوم/سوم شکست خورد و API محلی `502` داد. متن timeout متعلق به channel قدیمی AI Studio بود؛ بنابراین درخواست هنوز به `/backend-api/f/conversation` نرسیده بود. قدم بعدی ثبت trace امن در مرز dispatch و اصلاح routing است. تا قبل از مشاهده‌ی status واقعی conversation، این smoke موفق محسوب نمی‌شود.
