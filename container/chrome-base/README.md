# منبع image پایه Chrome

این پوشه snapshot فایل‌های build مربوط به image زیر است:

```text
moosavimaleki/ai-chrome-base:latest
sha256:1e8d88a80c26b1178ee347218ef6ddbac3ebe0de213944a761115e05a42acb8d
linux/amd64
created: 2025-12-13
```

منبع اولیه در زمان انتقال:

```text
/home/h-mousavi/Projects/Hamed/books/engine/ai_chrome/gui-docker
```

فایل `Dockerfile` بدون تغییر محتوایی از `Dockerfile.base` آورده شده است.
`requirements.txt` و `docker-entrypoint.sh` نیز ورودی‌های مستقیم همان build
هستند. فایل `Dockerfile` اپلیکیشن قدیمی و کدهای `src/` عمداً منتقل نشده‌اند،
چون سازندهٔ image پایه نبودند.

این snapshot برای گم‌نشدن منبع تاریخی نگه‌داری می‌شود. image جاری پروژه در
Dockerfile ریشه همچنان از tag منتشرشده استفاده می‌کند.

برای بازبینی build بدون push:

```bash
docker build \
  --file container/chrome-base/Dockerfile \
  --tag moosavimaleki/ai-chrome-base:local \
  container/chrome-base
```

سپس image اصلی پروژه را می‌توان با همین base محلی ساخت:

```bash
docker build \
  --build-arg CHROME_BASE_IMAGE=moosavimaleki/ai-chrome-base:local \
  --tag aistudio-api:local \
  .
```

اسکریپت قدیمی `base_builder.sh` عمداً منتقل نشده، چون بلافاصله پس از build هر
دو tag را روی Docker Hub push می‌کرد.
