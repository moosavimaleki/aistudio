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
`127.0.0.1:10808` فعال است و فقط به staging می‌رود.

The lab proxy is required. Before starting, verify that the host proxy is
available at `127.0.0.1:10808` and routes exclusively to staging.

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
      LAB_PROXY_URL: "http://host.docker.internal:10808"
      AISTUDIO_PROXY_URL: "http://host.docker.internal:10808"

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
      TOKEN_FACTORY_SAME_BROWSER_PROBE: "1"

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

### ۱. پیش‌نیازها

- فقط از session و endpointهای staging استفاده کنید.
- proxy آزمایشگاه باید روی میزبان فعال باشد و production را route نکند.
- Docker و Docker Compose باید نصب باشند.
- یک فایل Netscape cookie معتبر در `COOKIES/*.txt` قرار دهید.
- پوشه `COOKIES` باید برای کانتینر قابل‌نوشتن باشد، چون Chrome ممکن است
  cookieهای rotate‌شده را در همان فایل ذخیره کند.

هر فایل cookie یک profile مستقل می‌سازد. فایل اول `default`، فایل دوم
`browser2` و فایل سوم `browser3` است. برای انتخاب فایل دوم:

```yaml
AISTUDIO_DEFAULT_BROWSER_ID: "browser2"
AISTUDIO_AUTH_USER2: "0"
```

### ۲. اجرا و بررسی

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

### 1. Prerequisites

- Use staging sessions and staging endpoints only.
- The lab proxy must be running on the host and must never route to production.
- Install Docker and Docker Compose.
- Put a valid Netscape cookie file in `COOKIES/*.txt`.
- Keep the `COOKIES` mount writable because Chrome may persist rotated cookies
  back to the same Netscape file.

Each cookie file creates an independent browser profile. The first file is
`default`, the second is `browser2`, and the third is `browser3`. To select the
second file:

```yaml
AISTUDIO_DEFAULT_BROWSER_ID: "browser2"
AISTUDIO_AUTH_USER2: "0"
```

### 2. Start and verify

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
