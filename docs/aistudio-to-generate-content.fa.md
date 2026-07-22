# چرخهٔ AI Studio از session تا `GenerateContent`

## هدف

این سند توالی کامل مشاهده‌شده در آزمایشگاه staging را توضیح می‌دهد: از cookie session تا bootstrap، startup metadata، ساخت payload، Token Factory، streaming response، history و upload فایل.

دو نوع شاهد در این سند وجود دارد:

- **bundle/capture:** رفتار UI AI Studio و serializer داخلی؛
- **implementation:** رفتار تأییدشده در `src/aistudio_client` و `src/browser_interface`.

در مواردی که schema یا capability یک مدل قطعی نیست، client باید fail closed کند یا capture/feature test جدا داشته باشد.

## فلو end-to-end

```text
Netscape cookie export / configured session
  │
  ├── CookieJar و Authorization context
  │
  ├── GET AI Studio document (bootstrap)
  │     └── runtime API key + raw Visit ID + feature gates
  │
  ├── startup RPCها
  │     ├── GenerateAccessToken
  │     └── GetLoggingContext
  │
  ├── compose final MakerSuite positional payload
  │     ├── model / contents / safety / generation config
  │     ├── system instruction / tools / continuation context
  │     └── uploaded Drive file references
  │
  ├── content digest + TOKEN_FACTORY_URL
  │     └── fresh attestation token and possible state refresh
  │
  ├── POST MakerSuiteService/GenerateContent
  │     └── server-streaming response
  │
  └── preserve model parts + continuation state for next turn
```

## مرحلهٔ 0: cookie، account و origin

منبع cookie، فایل‌های Netscape با پسوند `.txt` در پوشهٔ `COOKIES/` است. هر فایل یک browser profile مستقل می‌سازد. parser فقط Google-domain cookieها را به یک `CookieJar` تبدیل می‌کند. همان CookieJar برای هر دو مورد استفاده می‌شود:

```text
Cookie header شبکه
Authorization وابسته به session/origin
```

این دو منبع نباید از دو فایل یا دو snapshot متفاوت بیایند؛ در غیر این صورت Token Factory و GenerateContent می‌توانند profileهای متفاوت ببینند.

origin فعلی برای این flow:

```text
https://aistudio.google.com
```

account index نیز بخشی از identity است. URL document برای account zero و accountهای دیگر متفاوت است:

```text
/prompts/new_chat
/u/<authUser>/prompts/new_chat
```

### failure قطعی session

این نشانه‌ها به معنی «cookie تازه لازم است» هستند:

- redirect صفحه به `accounts.google.com`؛
- نبودن markerهای runtime مورد انتظار در bootstrap؛
- ناتوانی در آماده‌شدن browser profile با همان cookie/authUser.

در این حالت API key یا Visit ID را hard-code نکنید؛ از cookie/session معتبر دوباره bootstrap کنید.

## مرحلهٔ 1: bootstrap document و runtime config

document AI Studio حامل config runtime است. implementation ابتدا page runtime را می‌خواند و در صورت نیاز HTML را fallback می‌گیرد. سه مفهوم استخراج می‌شود:

| مفهوم | منبع runtime | کاربرد |
|---|---|---|
| API key | `WIu0Nc` | `X-Goog-Api-Key` |
| raw visit id | `teM9xe` | `X-AIStudio-Visit-Id` پس از تبدیل نسخه‌دار |
| attestation feature gate | `UsvuEb` | فعال/غیرفعال بودن مسیر Token Factory |

Visit ID از raw ID با Base64URL و prefix `v1_` ساخته می‌شود. این ID session/cookie نیست؛ برای context و correlation visit است.

هم‌زمان، browser profile واقعی User-Agent، client hints و RPC identity metadata را مشاهده می‌کند. دلیل آن این است که API key/Visit ID document-owned هستند، اما `X-Client-Data` و browser identity از runtime browser می‌آیند.

## مرحلهٔ 2: Authorization و headerهای MakerSuite

GenerateContent با OAuth Bearer ارسال نمی‌شود. captureهای MakerSuite از Authorization session-hash همراه cookieهای همان session استفاده می‌کنند.

در سطح client:

```text
cookie material + normalized origin + current time
  → Authorization زمان‌دار
```

این مقدار قبل از هر request حساس دوباره ساخته می‌شود. علاوه بر آن، headerهای runtime لازم‌اند:

```text
Origin
Referer
Cookie
Authorization
User-Agent
X-Goog-Api-Key
X-Goog-AuthUser
X-AIStudio-Visit-Id
X-Client-Data و browser metadata
X-User-Agent: grpc-web-javascript/0.1
Content-Type: application/json+protobuf
```

هر header علت خاصی دارد: session identity، application identity، account selection، visit correlation یا browser transport identity. جایگزین‌کردن همه با capture ثابت به معنی session معتبر نیست.

## مرحلهٔ 3: startup RPCها و state قابل reuse

در `AIStudioTab.initialize()` مراحل زیر یک بار برای عمر virtual tab اجرا می‌شوند:

```text
bootstrap runtime config
  → GenerateAccessToken (اول)
  → GenerateAccessToken (دوم)
  → GetLoggingContext
  → tab READY
```

### GenerateAccessToken

این RPC OAuth access token کوتاه‌عمر می‌دهد. در capture، GenerateContent خود با session-hash Authorization اجرا می‌شود، نه Bearer token. کاربرد تأییدشدهٔ Bearer در flow فایل، Drive multipart upload است.

tab مقدار access token startup را نگه می‌دارد، اما باید expiry و account switch را در طراحی upload در نظر گرفت. نبودن استفادهٔ مستقیم آن در GenerateContent به معنی بی‌فایده‌بودن آن برای APIهای کمکی نیست.

### GetLoggingContext

این RPC metadata/extension مربوط به transport را تأمین می‌کند. پاسخ آن ممکن است state cookie تازه هم داشته باشد؛ به همین دلیل همهٔ responseها باید از CookieJar عبور کنند. context بازگشتی هنگام compose headerهای GenerateContent استفاده می‌شود.

### چه چیزهایی reuse می‌شوند؟

در یک tab سالم می‌توان این‌ها را reuse کرد:

```text
runtime API key و Visit ID
transport profile
LoggingContext
OAuth startup state، تا زمان expiry
cookie jar به‌روز
```

این‌ها نباید reuse شوند:

```text
attestation token GenerateContent
Authorization timestamp/hash قدیمی
content digest یک request دیگر
continuation token متعلق به گفتگو/session دیگر
```

## مرحلهٔ 4: مدل مفهومی request

client سطح بالا باید object معنی‌دار بپذیرد:

```python
GenerateInput(
    model="models/...",
    contents=[...],
    system_instruction="...",
    generation_config={...},
    safety_settings=[...],
    tools=[...],
    continuation_token=..., 
    tool_context=..., 
)
```

سپس `aistudio_client.makersuite` آن را به JSON positional داخلی تبدیل می‌کند. این مرز مهم است: Vertex-like input یک API model است؛ body آرایه‌ای MakerSuite implementation detail است.

### request در سطح اول

| index | نقش |
|---:|---|
| 0 | model |
| 1 | repeated contents |
| 2 | safety settings |
| 3 | generation config |
| 4 | attestation تازه؛ ابتدا خالی |
| 5 | system instruction |
| 6 | tools |
| 10 | flag داخلی مشاهده‌شده |
| 11 | continuation/pivot opaque از پاسخ قبلی |
| 13 | tool/search context، مانند timezone |

جزئیات Partها، config و schema در [makersuite-generate-content-lab.fa.md](makersuite-generate-content-lab.fa.md) آمده است.

## مرحلهٔ 5: ساخت contents و conversation state

هر Content:

```js
[
  [ /* Part[] */ ],
  "user" | "model"
]
```

Text Part:

```js
[null, "text"]
```

file reference Part پس از upload:

```js
[null, null, null, null, null, ["file-id"]]
```

برای history، raw Partهای model را preserve کنید. یک پاسخ model می‌تواند علاوه بر text، signature، thought، function result یا file reference داشته باشد. تبدیل history به `list[role, text]` فقط برای text-only demo مناسب است.

system instruction با history یکی نیست؛ encoder آن را در field جداگانه قرار می‌دهد، حتی اگر encoding داخلی‌اش role=`user` داشته باشد.

## مرحلهٔ 6: generation config و model capability

در schema داخلی تأیید شده:

```text
config[3] = maxOutputTokens
config[4] = temperature
config[5] = topP
config[6] = topK
config[7] = responseMimeType
config[8] = responseSchema
config[14] = responseModalities
config[15] = speechConfig
config[16] = thinkingConfig
config[17] = mediaResolution
config[18] = seed
```

نکات اجرایی:

- candidate count mapping داخلی تأیید نشده است؛ implementation آن را reject می‌کند.
- `responseSchema` باید به schema positional تبدیل شود؛ JSON Schema خام قابل ارسال نیست.
- thinking level string به enum داخلی نگاشت قطعی عمومی ندارد؛ implementation فقط `levelEnum` عددی می‌پذیرد.
- budget و level را هم‌زمان نفرستید.
- قابلیت واقعی هر feature model-dependent است؛ schema معتبر لزوماً به معنی مدل پشتیبان نیست.

## مرحلهٔ 7: digest و Token Factory

بعد از آن‌که payload نهایی، شامل file IDها و state گفتگو، ساخته شد:

در bundle، `_.bv()` تمام Partها را با `_.Di` project می‌کند. Text Part به متن و
file-reference Part در field 6 به `Drive fileId` تبدیل می‌شود؛ سپس مقادیر با یک
فاصله join و SHA-256 می‌شوند:

```text
projected Part values، شامل متن و Drive file ID
  → join با space
  → SHA-256 hex lowercase
  → TOKEN_FACTORY_URL
```

برای `[filePart, textPart]` ورودی hash برابر `drive-file-id متن درخواست` است.
حذف file ID از digest باعث می‌شود token ظاهراً ساخته شود، ولی GenerateContent
فایل را با 403 رد کند.

Token Factory context کامل GenerateContent را نیز می‌گیرد:

```text
URL و method
headers همان request
cookie header همان request
payload بدون field 5
browserId / authUser
```

سرویس browser-interface digest را دوباره محاسبه و identity را بررسی می‌کند. سپس extension همان browser profile token تازه می‌گیرد. پاسخ می‌تواند cookie records، runtime config یا transport profile به‌روز بدهد.

ترتیب درست client:

```text
1. payload بساز
2. digest بساز
3. token/context تازه بگیر
4. cookie updates را اعمال کن
5. Authorization را با cookie فعلی بساز
6. token را در payload[4] قرار بده
7. GenerateContent را بفرست
```

بازپیاده‌سازی provider یا token generation در client خارج از contract این آزمایشگاه است.

## مرحلهٔ 8: RPC و retry

request نهایی با `POST` و `application/json+protobuf` به RPC server-streaming ارسال می‌شود. `AIStudioTab` برای خطاهای گذرا مانند 408، 429، 500، 502، 503 و 504 retry محدود دارد.

هر retry token تازه می‌گیرد؛ retry با token field 5 قبلی ممنوع است. 401/403 معمولاً tab را invalid می‌کنند، زیرا نشانهٔ mismatch session/auth/attestation هستند. 429 معمولاً quota است و نباید tab سالم را نابود کند.

## مرحلهٔ 9: پاسخ streaming و turn بعدی

parser streaming باید این داده‌ها را نگه دارد:

```text
text chunks
raw model Parts
finish reason
usage
conversation metadata
continuation/pivot state
```

bundle continuation state پاسخ را در request بعدی برمی‌گرداند. این مقدار opaque است و فقط در همان session/گفتگو معتبر است. اگر caller conversation management دارد، آن را همراه raw history ذخیره کند.

AI Studio UI ممکن است بعد از inference `GenerateTitle` و `CreatePrompt`/`UpdatePrompt` بزند. این RPCها persistence/UI concern هستند، نه prerequisite اجرای GenerateContent توسط client آزمایشگاه.

## شاخهٔ upload فایل

### جریان مشاهده‌شده

```text
bytes + MIME
  → MakerSuiteService/GetAppFolder
  → OAuth Bearer access token
  → Google Drive multipart upload
  → Drive file ID
  → Part.fileData در payload GenerateContent
  → digest + Token Factory + GenerateContent
```

GenerateContent فایل bytes را دوباره نمی‌گیرد؛ فقط file ID را در Part field 6 می‌بیند.

### GetAppFolder

```text
POST MakerSuiteService/GetAppFolder
body: []
```

پاسخ folder اختصاصی app را می‌دهد. آن را hard-code نکنید و بین accountهای مختلف share نکنید.

### Drive multipart upload

در capture:

```text
POST content.googleapis.com/upload/drive/v3/files?uploadType=multipart&key=<runtime-key>
Authorization: Bearer <OAuth access token>
X-Goog-AuthUser: <same account>
Content-Type: multipart/related; boundary=<generated>
```

multipart شامل metadata JSON با `name` و `parents:[appFolderId]` و سپس bytes/base64 با MIME واقعی فایل است. پاسخ `drive#file` شامل `id` می‌دهد.

در payload:

```js
const filePart = [null, null, null, null, null, [uploadedFileId]];
```

تصویر، صوت و چند attachment در یک turn با Partهای متوالی دیده شده‌اند. `CountTokens` و `CheckImage` در UI قبل از بعضی مسیرهای media مشاهده شده‌اند، اما prerequisite قطعی همهٔ مدل‌ها/فایل‌ها اثبات نشده‌اند؛ آن‌ها را preflight model-specific بدانید.

پیاده‌سازی فعلی `aistudio_client` reference فایل را encode می‌کند، ولی API/worker upload خودکار را هنوز expose نمی‌کند. upload باید ماژول جدا با OAuth expiry، MIME validation و app folder state باشد.

## کدام بخش browser لازم دارد؟

| مرحله | browser واقعی لازم است؟ | دلیل |
|---|---|---|
| parse cookie export | خیر | عملیات local |
| Authorization/header composition | خیر | client state، با origin مشخص |
| bootstrap HTTP | خیر، در client ممکن است | document/runtime config |
| Chrome runtime/transport observation | بله | profile identity واقعی |
| lifecycle token snapshot | بله، پشت Token Factory | contract staging |
| GenerateContent HTTP | خیر، بعد از دریافت context معتبر | client transport |
| same-browser probe | فقط diagnostic | isolate کردن خطای token/context |
| Drive upload | خیر، اما OAuth/account context لازم دارد | HTTP API کمکی |

## ارجاع و وضعیت پیاده‌سازی

| قابلیت | bundle/capture | `aistudio_client` | public `gencontent` API |
|---|---|---|---|
| text-only | تأیید شده | پشتیبانی می‌شود | پشتیبانی می‌شود |
| history text | تأیید شده | پشتیبانی می‌شود | پشتیبانی می‌شود |
| raw Part history | تأیید شده | پشتیبانی می‌شود | expose نشده |
| system instruction | تأیید شده | پشتیبانی می‌شود | expose نشده |
| response schema | تأیید شده | subset پشتیبانی می‌شود | از `generationConfig` قابل عبور است |
| tools | تأیید شده | پشتیبانی می‌شود | expose نشده |
| continuation state | تأیید شده | پشتیبانی می‌شود | expose نشده |
| file reference | تأیید شده | پشتیبانی می‌شود | upload/reference API ندارد |
| Drive upload | تأیید شده | ماژول جدا لازم دارد | پیاده نشده |

## ارجاعات

- architecture سرویس: [botguard-microservice2-architecture.fa.md](botguard-microservice2-architecture.fa.md)
- schema positional و فایل: [makersuite-generate-content-lab.fa.md](makersuite-generate-content-lab.fa.md)
- تحلیل و شواهد تاریخی: [../GENERATE_CONTENT_ANALYSIS.fa.md](../GENERATE_CONTENT_ANALYSIS.fa.md) و [../GENERATE_CONTENT_SECURITY_FLOW.fa.md](../GENERATE_CONTENT_SECURITY_FLOW.fa.md)
