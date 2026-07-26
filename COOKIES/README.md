``# فایل‌های session مرورگر

در این پوشه برای هر حساب AI Studio یک فایل cookie با قالب
**Netscape HTTP Cookie File** و پسوند `.txt` قرار دهید. این فایل‌ها secret
هستند و نباید commit یا منتشر شوند.

نمونهٔ ساختار فایل:

```text
# Netscape HTTP Cookie File
.google.com	TRUE	/	TRUE	1893456000	SID	<value>
.google.com	TRUE	/	TRUE	1893456000	SAPISID	<value>
.google.com	TRUE	/	TRUE	1893456000	__Secure-1PAPISID	<value>
.google.com	TRUE	/	TRUE	1893456000	__Secure-3PAPISID	<value>
```

هر فایل باید session کامل و معتبر همان حساب را برای دامنه‌های Google داشته
باشد. اگر bootstrap به `accounts.google.com` redirect شد، cookieها ناقص یا
منقضی شده‌اند.

## ترتیب profileها

فایل‌ها با ترتیب طبیعی نام مرتب می‌شوند:

- فایل اول: `browserId=default` و `AISTUDIO_AUTH_USER`
- فایل دوم: `browserId=browser2` و `AISTUDIO_AUTH_USER2`
- فایل سوم: `browserId=browser3` و `AISTUDIO_AUTH_USER3`

برای انتخاب یک profile به‌عنوان مقصد پیش‌فرض:

```bash
AISTUDIO_DEFAULT_BROWSER_ID=browser2 docker compose up -d
```

Chrome ممکن است cookieهای session را هنگام اجرا rotate کند. کانتینر مقدار
جدید را در همین فایل Netscape ذخیره می‌کند؛ بنابراین mount این پوشه باید برای
سرویس `aistudio` قابل‌نوشتن باشد. در صورت خطای permission، گروه فایل‌ها را با
`COOKIE_WRITER_GID` تنظیم کنید.

فایل‌های `.txt` این پوشه توسط Git نادیده گرفته می‌شوند، ولی پیش از share کردن
repository یا logها حتماً نبودن cookie واقعی را دوباره بررسی کنید.
