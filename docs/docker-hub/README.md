# AI Studio API

Go gateway سازگار با Vertex که Chrome، افزونهٔ AI Studio، Token Factory و
GenerateContent را در یک کانتینر مدیریت می‌کند. Redis تنها سرویس جانبی است.

این image برای محیط staging/lab ساخته شده است. cookie و endpointهای آن را به
production متصل نکنید.

## اجرا با Docker Compose

یک پوشه بسازید و فایل‌های Netscape cookie را در `COOKIES/*.txt` قرار دهید.
سپس این `compose.yaml` را کنار پوشهٔ `COOKIES` ذخیره کنید:

```yaml
services:
  aistudio:
    image: YOUR_DOCKERHUB_USER/aistudio-api:latest
    ports:
      - "127.0.0.1:3345:3345"
      - "127.0.0.1:3346:8000"
      - "127.0.0.1:7900:7900"
    environment:
      PORT: "3345"
      GENCONTENT_PORT: "8000"
      FACTORY_ORIGIN: "http://127.0.0.1:3345"
      TOKEN_FACTORY_URL: "http://127.0.0.1:3345/get-token"
      AISTUDIO_COOKIE_DIR: "/app/cookies"
      REDIS_URL: "redis://redis:6379/0"
      LAB_PROXY_URL: "http://host.docker.internal:10808"
      AISTUDIO_PROXY_URL: "http://host.docker.internal:10808"
      TOKEN_FACTORY_SAME_BROWSER_PROBE: "1"
      AISTUDIO_AUTH_USER: "0"
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

اگر proxy میزبان روی پورت دیگری است، هر دو URL مربوط به proxy را تغییر دهید.
روی Linux نگه‌داشتن `extra_hosts` لازم است.

سرویس‌ها را بالا بیاورید:

```bash
docker compose up -d
docker compose ps
curl http://127.0.0.1:3345/health
curl http://127.0.0.1:3346/health
```

API اصلی روی پورت `3346` است. noVNC اختیاری روی
`http://127.0.0.1:7900` در دسترس است.

## چند profile

نام فایل‌ها ترتیب profileها را تعیین می‌کند. فایل اول `default`، فایل دوم
`browser2` و به همین ترتیب است. برای انتخاب فایل دوم:

```yaml
environment:
  AISTUDIO_DEFAULT_BROWSER_ID: "browser2"
  AISTUDIO_AUTH_USER2: "0"
```

cookieها هنگام rotate شدن توسط Chrome در همان فایل Netscape ذخیره می‌شوند؛
mount مربوط به `COOKIES` را read-only نکنید.

## انتشار image

در source repository:

```bash
scripts/prepare-release YOUR_DOCKERHUB_USER/aistudio-api 1.0.0 --load
docker image inspect YOUR_DOCKERHUB_USER/aistudio-api:1.0.0
```

پس از `docker login`، انتشار مستقیم:

```bash
scripts/prepare-release YOUR_DOCKERHUB_USER/aistudio-api 1.0.0 --push
```
