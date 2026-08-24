# کلاینت ChatGPT: معماری و Smoke Test

این قابلیت فقط برای تست end-to-end در آزمایشگاه staging است و نباید در
production یا برای دورزدن سامانه‌های ضدربات استفاده شود.

## دو مسیر مستقل

API دو رفتار صریح دارد:

| مدل API | سازنده و فرستندهٔ درخواست نهایی | کاربرد |
|---|---|---|
| `chatgpt-web` | صفحهٔ واقعی `chatgpt.com` در Chrome | تست کاملاً UI-based و تولید تصویر |
| `chatgpt/gpt-5.6` و مدل‌های هم‌خانواده | کلاینت Go | تست مستقیم schema، SSE و conversation API |

در مسیر مستقیم، Go payload و هر دو درخواست
`POST /backend-api/f/conversation/prepare` و
`POST /backend-api/f/conversation` را می‌سازد. مرورگر فقط artifactهای تازه و
وابسته به همان turn را با اجرای طبیعی frontend آماده می‌کند:

- session cookie و Authorization
- Sentinel prepare/proof و Turnstile، در صورت درخواست خود سایت
- headerهای واقعی prepare و final شامل client، build، device و session
- user agent، client hints و اطلاعات صفحه

شناسهٔ `x-oai-turn-trace-id` در Go یک بار برای هر turn ساخته و روی هر دو
درخواست استفاده می‌شود. `x-conduit-token` نیز خروجی prepare واقعی Go است.

هیچ challenge، Proof-of-Work، Turnstile یا Sentinel در Go حل، بازسازی یا
شبیه‌سازی نمی‌شود. artifactها cache یا reuse نیز نمی‌شوند.

## فلو مستقیم Go

```text
OpenAI-compatible request
        |
        v
Go: validate model/messages/conversation IDs
        |
        v
Chrome extension: submit a marked UI turn
        |
        v
native ChatGPT frontend: build prepare/final headers + protected challenges
        |
        v
extension captures prepare and final before network
and returns synthetic success responses to the UI
        |
        v
Go: fresh cookies + captured headers + current payloads
        |
        v
Chrome-compatible TLS/HTTP2 transport through the same proxy
        |
        v
/backend-api/f/conversation/prepare -> fresh conduit token
        |
        v
/backend-api/f/conversation -> SSE parser -> OpenAI response
```

TLS و HTTP/2 کلاینت Go با نزدیک‌ترین profile رسمی به Chrome 136 کانتینر، فعلاً Chrome 133،
ارسال می‌شود. User-Agent و client hint از Chrome واقعی کانتینر گرفته می‌شوند؛
این فقط رفتار transport مرورگر را در تست e2e بازتولید می‌کند و challenge solver
نیست. proxy مرورگر و Go باید یکی باشد تا session و IP از هم جدا نشوند.

## challenge و health

اگر صفحهٔ واقعی روی `Just a moment...` یا Turnstile بماند، profile آماده اعلام
نمی‌شود. `/health` در این حالت `503` و در فیلد `warmError` علت را گزارش می‌کند.
مرورگر نیز پی‌درپی restart نمی‌شود تا challenge طبیعی فرصت تکمیل داشته باشد.
پس از ظاهرشدن composer، monitor همان profile را بدون restart به حالت `READY`
برمی‌گرداند. API مستقیم تا آن زمان نیز سریعاً `503` می‌دهد.

## مدل‌ها

- `chatgpt/gpt-5.6`: مدل پایهٔ مستقیم
- `chatgpt/gpt-5.6-thinking`: حالت thinking با effort توسعه‌یافته
- `chatgpt/gpt-5.6-pro`: alias عمومی Pro روی قرارداد زندهٔ
  `gpt-5-6-thinking` با `thinking_effort=extended`
- `chatgpt-web`: مسیر کاملاً UI-based

نام‌های عمومی API از slug خصوصی upstream جدا هستند تا تغییر نام داخلی سایت
به قرارداد client نشت نکند.

## ادامهٔ conversation

Go state چت را نگه نمی‌دارد. در turn اول `conversation_id` خالی است؛ backend
شناسهٔ واقعی conversation و شناسهٔ پیام assistant را برمی‌گرداند. client باید
آن‌ها را برای turn بعدی ارسال کند:

```json
{
  "model": "chatgpt/gpt-5.6-pro",
  "conversation_id": "<id from previous response>",
  "parent_message_id": "<assistant message id from previous response>",
  "messages": [
    {"role": "user", "content": "Continue the same conversation"}
  ]
}
```

این شناسه‌ها در `lab_metadata.conversation_id` و
`lab_metadata.parent_message_id` پاسخ قرار دارند. در continuation فقط پیام
جدید ارسال می‌شود؛ history مالک backend ChatGPT است.

## Cookie و profile

هر فایل `CHATGPT_COOKIES/*.txt` یک Chrome مستقل می‌سازد. اولین فایل profile
`chatgpt`، فایل دوم `chatgpt2` و به همین ترتیب است. cookie اولیه از Netscape
وارد می‌شود و پس از bootstrap، profile دائمی Chrome مالک session و cookieهای
چرخیده است.

## نمونه‌ها

تست دو turn مستقیم:

```bash
CHATGPT_MODEL=chatgpt/gpt-5.6-pro python examples/chatgpt_openai.py
```

همان مثال با مسیر UI:

```bash
CHATGPT_MODEL=chatgpt-web python examples/chatgpt_openai.py
```

تولید تصویر فقط UI-based است:

```bash
python examples/chatgpt_image.py
```

## معیار Smoke معتبر

1. health مرورگر و gateway هر دو `200` باشند.
2. frontend واقعی prepare و challengeهای لازم را بدون خطای UI اجرا کند.
3. درخواست مستقیم Go از همان proxy با status `200` و SSE برگردد.
4. parser deltaهای کامل و فشردهٔ `p/o/v` را بخواند.
5. turn دوم با IDهای turn اول همان conversation را ادامه دهد.
6. prompt، cookie، Authorization و tokenهای محافظتی در log ثبت نشوند.

تست قرارداد Go، افزونه، race detector و `go vet` باید پیش از smoke زنده پاس
شوند. Smoke زنده تنها وقتی معتبر است که Chrome نیز `READY` باشد؛ عبور مصنوعی
از challenge نتیجهٔ آزمایش را نامعتبر می‌کند.
