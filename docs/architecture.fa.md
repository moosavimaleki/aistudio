# معماری Go

کد runtime پروژه فقط Go و JavaScript افزونه است:

```text
cmd/
  browser-interface/   API داخلی Chrome و Token Factory
  gencontent/          API سازگار با Vertex
  build-extension/     ساخت قطعی bundle افزونه
internal/
  aistudio/            قرارداد MakerSuite، auth، media و stream
  browserinterface/    fleet مرورگر، lifecycle و token broker
  chromeprocess/       اجرای Chrome و profileهای مستقل
  gencontent/          HTTP adapter، tab pool و upload
  metrics/             metric store و middleware
  extensionbuild/      builder افزونه
assets/extension/      source و تست‌های JavaScript افزونه
container/
  chrome-base/         snapshot منبع image پایه Chrome
  runtime/             entrypoint نهایی
  supervisor/          مدیریت processها و healthcheck
```

در کانتینر application، `supervisord` نقش PID 1 را دارد و Xvfb، x11vnc،
noVNC، browser-interface و gencontent را اجرا و restart می‌کند. Redis عمداً
سرویس جدا باقی مانده تا state leaseها lifecycle مستقل داشته باشد.

مسیر درخواست:

```text
Vertex HTTP → gencontent → browser-interface → extension → native provider
            → token تازه → MakerSuite GenerateContent → Vertex response
```

Chrome مالک session و cookie jar زنده است. browser-interface بعد از هر snapshot
cookieهای rotate‌شده را در فایل Netscape همان profile ذخیره می‌کند تا
gencontent و Chrome از یک session material استفاده کنند.

کدهای Python قدیمی حذف شده‌اند. فایل‌های `examples/*.py` فقط clientهای تست
خارج از image هستند و در Docker build کپی نمی‌شوند.
