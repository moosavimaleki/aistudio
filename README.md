# AI Studio API

> [!CAUTION]
> **فقط برای آزمایشگاه متصل به staging — STAGING LAB ONLY**
>
> استفاده از این image در production، با حساب production، یا خارج از محیط
> آزمایشگاهی کنترل‌شده ممنوع است. حتی در آزمایشگاه، تمام upstreamها، Token
> Factory و proxy باید منحصراً به staging هدایت شوند.
>
> Never use this image in production, with production accounts, or outside a
> controlled lab. Even in a lab, every upstream route, Token Factory, and proxy
> must resolve exclusively to staging.

Go gateway سازگار با Vertex که Chrome، افزونه AI Studio، Token Factory و
GenerateContent را در یک کانتینر اجرا می‌کند. Redis تنها سرویس جانبی است.

A Vertex-compatible Go gateway running Chrome, the AI Studio extension, Token
Factory, and GenerateContent in one container. Redis is the only side service.

## نمونه کامل Docker Compose / Complete Docker Compose example

اتصال به proxy آزمایشگاه الزامی است. پیش از اجرا مطمئن شوید proxy میزبان روی
`0.0.0.0:10811` فعال است و فقط به staging می‌رود.

The lab proxy is required. Before starting, verify that the host proxy is
available at `0.0.0.0:10811` and routes exclusively to staging.

```yaml
services:
  aistudio:
    image: moosavimaleki/aistudio-api:1.0.0
    ports:
      - "127.0.0.1:3345:3345"
      - "127.0.0.1:3346:8000"
      - "127.0.0.1:5900:5900"
      - "127.0.0.1:7900:7900"
    environment:
      # Process ports
      PORT: "3345"
      GENCONTENT_PORT: "8000"

      # Internal service topology
      FACTORY_ORIGIN: "http://127.0.0.1:3345"
      TOKEN_FACTORY_URL: "http://127.0.0.1:3345/get-token"
      REDIS_URL: "redis://redis:6379/0"

      # REQUIRED: both must point to the staging lab proxy
      LAB_PROXY_URL: "http://host.docker.internal:10811"
      AISTUDIO_PROXY_URL: "http://host.docker.internal:10811"

      # AI Studio profile settings
      AISTUDIO_MODEL: "models/gemini-3.5-flash-lite"
      AISTUDIO_COOKIE_DIR: "/app/cookies"
      AISTUDIO_AUTH_USER: "0"
      AISTUDIO_AUTH_USER2: "0"
      AISTUDIO_DEFAULT_BROWSER_ID: ""

      # Chrome runtime
      CHROME_EXECUTABLE: "/usr/bin/google-chrome"
      CHROME_RUNTIME_DIR: "/app/browser-profiles"
      EXTENSION_SOURCE_DIR: "/app/extension"
      AISTUDIO_RUNTIME_DIR: "/app/runtime/state"
      CHROME_CDP_BASE_PORT: "9223"
      EXPECTED_BROWSER_MAJOR: ""
      AISTUDIO_PAGE_READY_TIMEOUT_MS: "60000"

      # GenerateContent tab pool
      TAB_POOL_MAX: "100"
      TAB_POOL_WAIT_SECONDS: "5"
      TAB_POOL_LEASE_SECONDS: "600"
    extra_hosts:
      - "host.docker.internal:host-gateway"
    volumes:
      - ./COOKIES:/app/cookies
      - browser_profiles:/app/browser-profiles
    group_add:
      - "${COOKIE_WRITER_GID:-1000}"
    shm_size: "1gb"
    restart: unless-stopped
    depends_on:
      redis:
        condition: service_healthy

  redis:
    image: redis:7-alpine
    command: ["redis-server", "--save", "", "--appendonly", "no"]
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 2s
      timeout: 2s
      retries: 20

volumes:
  browser_profiles:
```

## راهنمای فارسی

### ۱. نصب Docker و آماده‌سازی proxy

- Docker و Docker Compose را نصب کنید.
- proxy آزمایشگاه را روی میزبان، روی `0.0.0.0:10811`، بالا بیاورید.
- مطمئن شوید proxy فقط به staging route می‌کند و به production وصل نمی‌شود.

### ۲. استخراج Cookie از Chrome

۱. در Chrome وارد حساب آزمایشگاهی AI Studio شوید و مطمئن شوید صفحهٔ
   `https://aistudio.google.com` بدون redirect به صفحهٔ ورود باز می‌شود.
۲. افزونهٔ [Get cookies.txt LOCALLY](https://chromewebstore.google.com/detail/get-cookiestxt-locally/cclelndahbckbenkjhflpdbgdldlbecc)
   را از Chrome Web Store نصب کنید.
۳. در همان تب AI Studio، روی آیکن افزونه کلیک کنید و خروجی را با قالب
   **Netscape HTTP Cookie File** ذخیره کنید.
۴. فایل خروجی را با پسوند `.txt` داخل پوشهٔ `COOKIES/` پروژه قرار دهید؛ برای
   نمونه `COOKIES/work.txt` یا `COOKIES/work2.txt`.
۵. فایل Cookie را commit، publish یا در logها چاپ نکنید. این فایل معادل session
   مرورگر است و باید فقط برای staging استفاده شود.

هر فایل Cookie یک profile مستقل می‌سازد. فایل اول `default`، فایل دوم
`browser2` و فایل سوم `browser3` است. پوشهٔ `COOKIES` باید برای کانتینر
قابل‌نوشتن باشد، چون Chrome ممکن است Cookieهای rotate‌شده را در همان فایل
ذخیره کند.

برای انتخاب فایل دوم:

```yaml
AISTUDIO_DEFAULT_BROWSER_ID: "browser2"
AISTUDIO_AUTH_USER2: "0"
```

### ۳. اجرا و بررسی

فایل بالا را با نام `compose.yaml` ذخیره و اجرا کنید:

```bash
docker compose up -d
docker compose ps
curl http://127.0.0.1:3345/health
curl http://127.0.0.1:3346/health
```

- API اصلی: `http://127.0.0.1:3346`
- Browser/Token Factory API: `http://127.0.0.1:3345`
- noVNC اختیاری: `http://127.0.0.1:7900`

اگر proxy روی پورت دیگری است، هر دو متغیر `LAB_PROXY_URL` و
`AISTUDIO_PROXY_URL` را با هم تغییر دهید. روی Linux، مقدار `extra_hosts` را
حذف نکنید.

## English guide

### 1. Install Docker and prepare the proxy

- Install Docker and Docker Compose.
- Start the lab proxy on the host at `0.0.0.0:10811`.
- Verify that the proxy routes only to staging and never to production.

### 2. Export cookies from Chrome

1. Sign in to the laboratory AI Studio account in Chrome and verify that
   `https://aistudio.google.com` opens without redirecting to sign-in.
2. Install [Get cookies.txt LOCALLY](https://chromewebstore.google.com/detail/get-cookiestxt-locally/cclelndahbckbenkjhflpdbgdldlbecc)
   from the Chrome Web Store.
3. On the AI Studio tab, click the extension icon and export the cookies in
   **Netscape HTTP Cookie File** format.
4. Put the exported `.txt` file in the project `COOKIES/` directory, for
   example `COOKIES/work.txt` or `COOKIES/work2.txt`.
5. Never commit, publish, or print the cookie file in logs. It represents the
   browser session and is staging-only.

Each cookie file creates an independent browser profile. The first file is
`default`, the second is `browser2`, and the third is `browser3`. To select the
second file:

```yaml
AISTUDIO_DEFAULT_BROWSER_ID: "browser2"
AISTUDIO_AUTH_USER2: "0"
```

### 3. Start and verify

Save the complete example above as `compose.yaml`, then run:

```bash
docker compose up -d
docker compose ps
curl http://127.0.0.1:3345/health
curl http://127.0.0.1:3346/health
```

- Main API: `http://127.0.0.1:3346`
- Browser/Token Factory API: `http://127.0.0.1:3345`
- Optional noVNC UI: `http://127.0.0.1:7900`

If your staging proxy uses another port, change both `LAB_PROXY_URL` and
`AISTUDIO_PROXY_URL` together. On Linux, keep the `extra_hosts` mapping.

## انتشار / Publishing

```bash
docker login
scripts/prepare-release moosavimaleki/aistudio-api 1.0.0 --push
```
