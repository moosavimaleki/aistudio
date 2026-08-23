# معماری عملیاتی `src`

## هدف

`src` محیط عملیاتی staging برای اجرای end-to-end GenerateContent است. هدف آن جایگزین‌کردن AI Studio نیست؛ وظیفه‌اش این است که client HTTP، browser identity واقعی و Token Factory staging را به یک جریان قابل‌تست و قابل‌مقیاس تبدیل کند.

ویژگی‌های اصلی:

- پشتیبانی از چند browser profile با cookie و `authUser` مستقل؛
- lifecycle واقعی Chrome و extension مورد تأیید staging؛
- دریافت attestation فقط از Token Factory؛
- حفظ state قابل‌استفادهٔ client در Redis؛
- اجرای GenerateContent بدون UI قابل‌نمایش، اما با browser-compatible runtime؛
- خطاهای تشخیصی با phase و response body در محیط آزمایشگاه.

## نمای معماری

```text
Caller / CI
   │ POST /generate-content
   ▼
gencontent :3346
   │
   ├── RedisTabPool ─────── Redis
   │       │
   │       ▼
   │   AIStudioTab (Python client)
   │       │       ├── cookies / Authorization / runtime config
   │       │       ├── GetLoggingContext / startup OAuth state
   │       │       └── MakerSuite request + streaming parser
   │       │
   │       ▼ POST /get-token
   │
browser-interface :3345
   │
   ├── BrowserFleet
   │    ├── isolated Chrome process: browser profile A
   │    ├── isolated Chrome process: browser profile B
   │    └── ...
   │
   ├── BrowserSession
   │    ├── cookie injection and account navigation
   │    ├── runtime / transport observation
   │    └── native lifecycle warm-up
   │
   ├── TokenBroker  ◄────► staging bridge extension
   │
   └── TokenService
        └── request validation + fresh token + rotated state
```

در compose فعلی، `browser-interface` به پورت host `3345` و `gencontent` به پورت host `3346` نگاشت می‌شوند. Redis داخلی، state pool را نگه می‌دارد. این پورت‌ها را با Token Factory قدیمی روی `3344` یکی نگیرید؛ هر استقرار باید URL خودش را از environment بخواند.

## مسئولیت هر جزء

### `gencontent`

مسیر public این سرویس `POST /generate-content` است. وظیفهٔ آن orchestration است، نه browser automation:

1. input API را validate می‌کند.
2. یک virtual tab از Redis lease می‌گیرد.
3. برای tab جدید، browser profile آماده انتخاب می‌کند؛ برای tab موجود، profile ذخیره‌شده را حفظ می‌کند.
4. `AIStudioTab` را initialize یا restore می‌کند.
5. GenerateContent را اجرا می‌کند.
6. در موفقیت، snapshot tab را به pool بازمی‌گرداند؛ در خطای invalidating tab، lease را discard می‌کند.

پیاده‌سازی مرجع: `gencontent/service.py` و `gencontent/pool/`.

API public فعلی به‌صورت عمدی کوچک است:

```json
{
  "prompt": "required",
  "model": "optional",
  "history": [],
  "generationConfig": {},
  "safetySettings": []
}
```

لایهٔ داخلی `aistudio_client` از `contents`، `systemInstruction`، `tools`، continuation token و tool context نیز پشتیبانی دارد. expose کردن آن‌ها در API `gencontent` یک تغییر محصول جداگانه است و نباید با حدس یا pass-through بدون validation انجام شود.

### `aistudio_client.AIStudioTab`

virtual tab یک state machine Python است، نه tab واقعی Chrome. این شیء مالک stateهای هم‌بستهٔ زیر است:

```text
cookie jar
Authorization context
runtime API key + Visit ID
transport profile
LoggingContext extension
OAuth startup state
Token Factory binding
conversation/request state
```

stateهای آن:

```text
NEW → INITIALIZING → READY → GENERATING → READY
                         │              │
                         └──── INVALID / FAILED
```

در `READY` می‌تواند در Redis serialise شود. HTTP session worker-local serialise نمی‌شود؛ هنگام restore یک HTTP client تازه ساخته می‌شود، ولی cookie/runtime/transport state بازیابی می‌شود.

### `browser-interface`

این سرویس authority مرورگر است. مسئولیت آن:

- ساخت و نگه‌داری Chromeهای ایزوله؛
- اعمال cookieهای Google و بررسی identity؛
- رفتن به URL حساب انتخاب‌شدهٔ AI Studio؛
- استخراج runtime config صفحه و مشاهدهٔ headerهای واقعی یک RPC؛
- اطمینان از اتصال extension؛
- warm کردن lifecycle native؛
- صف‌کردن job token برای extension؛
- validate کردن اینکه درخواست Token Factory متعلق به همان GenerateContent و همان browser identity است؛
- بازگرداندن token و state تازهٔ browser به caller.

این سرویس encoder درخواست MakerSuite نیست. به همین دلیل browser-interface نباید generation config، history یا tool schema را خودش بسازد.

### staging bridge extension و `TokenBroker`

TokenBroker یک queue داخلی دارد:

```text
TokenService → broker.request(...)
             → GET /internal/jobs/next?browserId=...
             → extension در Chrome profile درست
             → POST /internal/jobs/{id}/result
             → TokenService
```

هر job با `browserId` route می‌شود. این نکته مانع دریافت token یک account توسط GenerateContent account دیگر می‌شود.

extension تنها بخشی است که lifecycle مورد تأیید staging را در realm مرورگر دریافت می‌کند. client و gencontent صرفاً contract آن را مصرف می‌کنند.

### Redis tab pool

`RedisTabPool` lease اتمیک می‌دهد و این stateها را نگه می‌دارد:

```text
tab id
lease token و expiry
browserId / authUser
cookie header و runtime config
transport profile / LoggingContext
generate count و feature state
```

pool دارای محدودیت size، waiting timeout و lease expiry است. در نتیجه یک tab هم‌زمان به دو درخواست داده نمی‌شود. quota upstream مستقل از این قفل است و باید با rate limit جدا کنترل شود.

## lifecycle یک Browser Profile

هر profile از یک `BrowserSpec` می‌آید:

```text
browserId
authUser
cookie source
```

هر فایل Netscape با پسوند `.txt` در پوشهٔ `COOKIES/` یک profile جدا می‌سازد. ترتیب طبیعی نام فایل‌ها profile اول، دوم و بعدی را تعیین می‌کند. هر profile Chrome process، CDP port و BrowserSession خودش را دارد. `AISTUDIO_AUTH_USER` و متغیرهای شماره‌دار متناظر، account index همان profile هستند و مقدار پیش‌فرض هرکدام `0` است.

### آماده‌سازی session

```text
load Cookie source
  → parse only Google-domain records
  → launch/connect isolated Chrome
  → clear prior cookies
  → inject and verify cookies
  → navigate to /u/<authUser>/prompts/new_chat
  → read runtime config
  → read browser transport profile
  → wait for extension connection
  → run native lifecycle warm-up
  → observe a MakerSuite RPC identity
  → mark BrowserSession READY
```

اگر navigation به `accounts.google.com` redirect شود، `InvalidCookieSession` است: cookieها منقضی، ناقص یا متعلق به account دیگری هستند. اگر runtime config فاقد markerهای API key و raw Visit ID باشد نیز همین نتیجهٔ عملی را دارد؛ باید cookie تازه داده شود، نه اینکه مقدار ثابت invent شود.

### runtime config و transport profile

BrowserSession از page runtime سه مفهوم را می‌خواند:

```text
runtime API key
raw visit identifier → formatted Visit ID
feature gate attestation enabled/disabled
```

هم‌زمان headerهای identity از Chrome واقعی گرفته می‌شوند: User-Agent، client hints، `X-Client-Data` و browser metadata. سپس `RpcObserver` یک MakerSuite RPC واقعی را مشاهده می‌کند تا profile با رفتار همان browser هماهنگ شود.

دلیل این دو منبع جدا:

- runtime config متعلق به document AI Studio است؛
- transport profile متعلق به browser/network identity است.

هیچ‌کدام از cookie استخراج نمی‌شوند.

## جریان Token Factory

### ورودی معتبر

TokenService فقط وقتی token درخواست می‌کند که این‌ها با هم سازگار باشند:

```text
digest معتبر SHA-256
cookie header غیرخالی
Authorization غیرخالی
WAA API key staging
GenerateContent context: URL + POST + payload
headerهای لازم: origin, cookie, authorization, user-agent,
                 x-client-data, x-goog-api-key, x-goog-authuser
```

service digest را دوباره از content projection payload محاسبه می‌کند. همچنین cookie و `authUser` را با BrowserSession انتخاب‌شده match می‌کند. این validation برای جلوگیری از جدا شدن token context از request نهایی است.

### خروجی

پاسخ token ممکن است شامل این‌ها باشد:

```text
token تازه
cookieRecords تازه
transportProfile تازه
runtimeConfig تازه
loggingContextExtension تازه
browserId
authUser
```

caller باید cookie records را پیش از ساخت Authorization و RPC اعمال کند. token صرفاً برای همان GenerateContent تازه است؛ reuse آن برای retry یا request جدید مجاز نیست.

### مرز درخواست کاربر

payload مربوط به caller هیچ‌وقت برای probe یا inference از داخل Chrome ارسال نمی‌شود. Chrome فقط provider بومی را نگه می‌دارد و برای digest همان درخواست token تازه می‌سازد؛ RPC نهایی را client مستقل Go ارسال می‌کند.

## native lifecycle warm-up

warm-up در `internal/browserinterface/lifecycle.go` انجام می‌شود:

1. UI page آماده می‌شود و در صورت نمایش consent، تأیید می‌گردد.
2. یک prompt داخلی آماده‌سازی در textbox قرار می‌گیرد.
3. submit native کلیک می‌شود.
4. route مربوط به GenerateContent intercept می‌شود تا headerهای native ثبت شوند.
5. request داخلی warm-up ادامه پیدا می‌کند تا UI نیز پاسخ عادی بگیرد و provider کامل آماده شود.

این تنها inference مرورگری است و صرفاً هنگام warm-up با متن ثابت آزمایشگاه اجرا می‌شود. payload caller در این مسیر استفاده نمی‌شود و وارد history virtual tab نیز نخواهد شد.

## APIهای عملیاتی

### browser-interface

| مسیر | هدف |
|---|---|
| `GET /health` | وضعیت connection، ready بودن browserها و jobهای pending |
| `POST /bootstrap` | آماده‌سازی یا refresh BrowserSession برای profile انتخابی |
| `POST /get-token` | دریافت token تازه برای GenerateContent معتبر |
| `GET /internal/jobs/next` | polling extension برای job profile مشخص |
| `POST /internal/jobs/{id}/result` | بازگرداندن نتیجهٔ extension |

### gencontent

| مسیر | هدف |
|---|---|
| `GET /health` | وضعیت Redis pool و browser profileهای آماده |
| `GET /` | dashboard تشخیصی pool/profile |
| `POST /generate-content` | inference end-to-end از طریق virtual tab |

## رفتار خطا و recovery

| نشانه | برداشت عملی | اقدام |
|---|---|---|
| redirect به accounts.google.com | cookie session نامعتبر/ناقص | cookie تازه، سپس bootstrap جدید |
| نبودن runtime markers | معمولاً cookie یا page state نامعتبر | cookie تازه؛ مقدار hard-code نکنید |
| extension disconnected | lifecycle browser profile آماده نیست | reload/restart profile یا extension، health را بررسی کنید |
| 401/403 RPC | auth/attestation/session mismatch | tab را invalid و profile context را بررسی کنید |
| 429 | quota upstream | retry محدود یا rate limit؛ tab را به‌خاطر quota discard نکنید |
| lease expiry | worker بیش از مدت lease فعال بوده | tab را دوباره materialize کنید |
| 400 invalid argument | model/schema/input mismatch | encoder و capability مدل را بررسی کنید |

## مرزهای فعلی implementation

پیاده‌سازی موجود عمداً این کارها را انجام نمی‌دهد:

- ساخت یا مهندسی معکوس provider attestation در client؛
- ساخت خودکار file upload در `gencontent` API؛
- expose کردن مستقیم raw positional payload به API public؛
- تبدیل حدسی string thinking level به enum داخلی؛
- نگهداری هم‌زمان یک virtual tab توسط چند request.

این‌ها محدودیت نیستند که با hard-code سریع رفع شوند؛ هر تغییر باید یک contract، validation و golden test capture داشته باشد.

## اجرای چند account

برای ده cookie مستقل، ده `BrowserSpec`/Chrome process با `browserId` جدا بسازید. profile مشترک برای accountهای مختلف مناسب نیست، چون cookie jar، runtime config، transport identity، extension job و token lifecycle باید به همان profile pin شوند.

برای هر request، انتخاب profile در اولین materialize tab انجام می‌شود و در snapshot tab ذخیره می‌شود. requestهای بعدی همان tab باید همان `browserId` و `authUser` را نگه دارند.

## ارجاعات

- چرخهٔ کامل protocol: [aistudio-to-generate-content.fa.md](aistudio-to-generate-content.fa.md)
- schema و upload: [makersuite-generate-content-lab.fa.md](makersuite-generate-content-lab.fa.md)
- security history: [../GENERATE_CONTENT_SECURITY_FLOW.fa.md](../GENERATE_CONTENT_SECURITY_FLOW.fa.md)
