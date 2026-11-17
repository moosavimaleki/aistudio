# استقرار سرور و انتقال session

این پروژه برای آزمایشگاه staging است. Cookieها را در Git، image یا log قرار
ندهید. توکن‌های attestation کوتاه‌عمر هستند و باید روی خود سرور از Token Factory
ساخته شوند؛ چیزی که بین سیستم محلی و سرور جابه‌جا می‌شود فایل Netscape cookie
است.

## توکن API دامنهٔ سرور

از ۲۰۲۶-۱۰-۲۸، روی `https://aistudio.shekarchi-app.ir` مسیرهای `/v1`، `/v1/*`
و `/generate-content` با Bearer token محافظت می‌شوند. توکن را از مالک سرویس
دریافت کنید و در محیط کلاینت قرار دهید؛ مقدار آن در Git ذخیره نشده است:

```bash
curl -sS https://aistudio.shekarchi-app.ir/v1/models \
  -H "Authorization: Bearer $AISTUDIO_API_TOKEN"
```

توکن اشتباه یا مفقود HTTP 401 می‌گیرد. این کنترل در Caddy سرور، در فایل
`/etc/caddy/private/aistudio-api-auth.caddy` با دسترسی `root:caddy 640` است؛
فایل دامنه در `/etc/caddy/conf.d/aistudio.shekarchi-app.ir.caddy` آن را import
می‌کند. تغییر آن نیاز به `caddy validate` و reload دارد، نه rebuild کانتینر.
بازسازی Docker توسط اسکریپت sync این کنترل را حذف نمی‌کند.
این توکن مربوط به API همین دامنه است، نه LiteLLM لوکال یا upstreamها.
داشبورد `/` و health در این تغییر محافظت نشدند. API لوکال 3346 و مسیر داخلی
Docker نیز تغییری نکردند و نباید مستقیماً عمومی شوند. موفقیت احراز هویت به
معنی سالم‌بودن Chrome session یا موفقیت تولید نیست.

## روش پیشنهادی

روی سرور یک checkout از پروژه داشته باشید و Compose را همان‌جا اجرا کنید. API همچنان
فقط روی localhost است؛ VNC برای اتصال مستقیم Remmina روی پورت `4314` منتشر می‌شود.
پیش از اجرا، `AISTUDIO_PROXY_URL` و `LAB_PROXY_URL` را در `.env` سرور به proxy
آزمایشگاهِ قابل‌دسترسی از همان سرور تنظیم کنید؛ proxy سیستم محلی خودکار به سرور
منتقل نمی‌شود.

```bash
cd /srv/aistudio-api
docker compose up -d --build
docker compose ps
```

از سیستم محلی، بعد از قرار دادن cookieهای AI Studio در `COOKIES/*.txt`، آن‌ها را
با یک دستور منتقل کنید:

```bash
scripts/sync_sessions_to_server.sh \
  --ssh-port 9011 \
  deploy@your-server:/srv/aistudio-api
```

این دستور فقط فایل‌های `.txt` دو پوشهٔ cookie را با SSH/rsync می‌فرستد، permission
را محدود می‌کند و سرویس `aistudio` را با `--force-recreate` دوباره می‌سازد تا Chrome
profileها از نو با sessionهای جدید بالا بیایند. مقدار cookie چاپ نمی‌شود.

برای استخراج خودکار sessionهای سالم ChatGPT از Chrome محلی، گزینهٔ زیر را اضافه
کنید:

```bash
scripts/sync_sessions_to_server.sh --import-chatgpt \
  --ssh-port 9011 \
  deploy@your-server:/srv/aistudio-api
```

به‌صورت پیش‌فرض فایل‌های قدیمی سرور حذف نمی‌شوند. اگر می‌خواهید سرور دقیقاً آینهٔ
دو پوشهٔ محلی باشد، `--prune` را آگاهانه اضافه کنید:

```bash
scripts/sync_sessions_to_server.sh --prune \
  --ssh-port 9011 \
  deploy@your-server:/srv/aistudio-api
```

## اجرای خودکار ساعتی از کامپیوتر محلی

مدیر سرویس را **بدون sudo و با همان کاربر Chrome** روی سیستم لوکال اجرا کنید:

```bash
cd /home/h-mousavi/Projects/Hamed/aistudio-api
python3 scripts/manage_session_sync.py
```

منوی عددی:

1. نصب/به‌روزرسانی اجرای ساعتی
2. اجرای فوری همگام‌سازی
3. ری‌استارت تایمر (بدون قطع‌کردن انتقال در حال اجرا)
4. توقف زمان‌بندی
5. وضعیت سرویس و زمان اجرای بعدی
6. نمایش لاگ‌ها
7. حذف سرویس و تنظیمات آن؛ بدون حذف هیچ کوکی یا فایل پروژه
0. خروج

هنگام نصب، مقصد پیش‌فرض `root@87.248.139.241:/opt/aistudio-api` و پورت پیش‌فرض
`9011` نمایش داده می‌شود؛ با Enter تأیید کنید یا مقدار دیگری بدهید.
نصب باعث راه‌اندازی تایمر systemd مخصوص کاربر می‌شود، نه سرویس root.
اجرای ساعتی ابتدا sessionهای سالم ChatGPT را از Chrome استخراج می‌کند، سپس
کوکی‌های دو پوشه را می‌فرستد و فقط پس از موفقیت هر دو انتقال، کانتینر سرور را
با `--force-recreate` از نو بالا می‌آورد. کوکی Google خودکار از Chrome استخراج
نمی‌شود؛ منبع آن همان فایل‌های موجود `COOKIES/*.txt` است.

پیش‌نیازها: Linux/systemd، Bash، SSH، rsync، flock و Python دارای `yt-dlp`.
باید قبل از نصب، این SSH **بدون درخواست رمز/تأیید host key** کار کند:

```bash
ssh -p 9011 root@87.248.139.241 true
ssh -o BatchMode=yes -o StrictHostKeyChecking=yes -p 9011 root@87.248.139.241 true
```

کلید و رمز SSH داخل unit ذخیره نمی‌شود. از کلید موجود یا ssh-agent استفاده
می‌شود. اگر socket مربوط به agent تغییر کرد، گزینهٔ نصب/به‌روزرسانی را مجدد بزنید.
بازبودن keyring دسکتاپ برای خواندن کوکی‌های Chrome لازم است؛ سرویس نیاز به
`DBUS_SESSION_BUS_ADDRESS` دارد و نصب متغیرهای موجود دسکتاپ/agent را به systemd
کاربر معرفی می‌کند. اگر proxy اعتبارسنجی ChatGPT لازم است، قبل از اجرا محیط
مناسب را در user manager تنظیم کنید؛ proxy پیش‌فرض importer محلی `10809` است:

```bash
systemctl --user set-environment CHATGPT_PROXY_URL=http://127.0.0.1:10809
```

با خاموشی/خواب سیستم انتقال انجام نمی‌شود. `Persistent=true` یک اجرای ازدست‌رفته
را بعد از فعال‌شدن user manager جبران می‌کند؛ نصب مجدد هم ممکن است آن را فوری
فعال کند. ساعت اجرا سر هر ساعت است، نه دقیقاً یک ساعت بعد از پایان قبلی.
پیش‌فرض ماندگاری بعد از logout را تغییر نمی‌دهیم؛ فعال‌کردن linger به‌تنهایی
تضمین نمی‌کند keyring قفل‌شده در دسترس باشد. زمان اجرای بعدی در گزینهٔ ۵ مشخص است.

فرمان غیرتعاملی نصب، فقط برای فایل‌های کوکی موجود و بدون استخراج تازه:

```bash
python3 scripts/manage_session_sync.py install \
  --target root@87.248.139.241:/opt/aistudio-api --port 9011 \
  --no-import-chatgpt
```

سایر فرمان‌ها بدون منو نیز قابل اجرا هستند:

```bash
python3 scripts/manage_session_sync.py run
python3 scripts/manage_session_sync.py restart
python3 scripts/manage_session_sync.py status
python3 scripts/manage_session_sync.py logs
python3 scripts/manage_session_sync.py stop
python3 scripts/manage_session_sync.py uninstall
```

فایل‌ها در `~/.config/systemd/user/aistudio-session-sync.{service,timer}` و
تنظیم مقصد در `~/.config/aistudio-session-sync/settings.json` قرار می‌گیرند.
مالکیت فایل آپلودشده از پوشهٔ کوکی سرور گرفته می‌شود؛ نباید کوکی‌ها root-owned
شوند وقتی مرورگر با کاربر دیگری اجرا می‌شود. قفل `flock` اجرای هم‌زمان دستی و
سرویسی را skip می‌کند. حذف سرویس در حین انتقال پذیرفته نمی‌شود؛ منتظر پایانش بمانید.
هر اجرای موفق، سرویس مرورگر سرور را موقتاً قطع می‌کند؛ درخواست‌های در حال انجام
ممکن است شکست بخورند. این رفتار مطابق درخواست restart ساعتی است، نه deploy نسخهٔ جدید.

API هنگام SIGTERM درخواست‌های در حال اجرا را cancel می‌کند و تا ۵ ثانیه برای
خروج handlerها صبر می‌کند. آزادسازی قفل conversation/bootstrap مستقل از cancel
درخواست و با timeout جداگانه انجام می‌شود؛ در نتیجه قطع کلاینت یا restart عادی
نباید pool را تا پایان lease سی‌دقیقه‌ای مشغول نگه دارد. در SIGKILL یا قطع برق،
انقضای lease همچنان مرز بازیابی قفل است.

خطای `ChatGPT did not return to a clean chat after the probe` می‌تواند از انتخاب
دکمهٔ مخفی `New chat` باشد؛ افزونه فقط دکمهٔ قابل‌مشاهده را انتخاب می‌کند. خطای
آماده‌سازی پروفایل خودکار با HTTP 504 نیز مانند 503 مسیر pool را دوباره انتخاب
می‌کند. timeout تولیدِ در حال اجرا به‌تنهایی باعث ارسال مجدد پیام نمی‌شود.

### خطای «صفر session سالم» در importer

ساختار جدید `/backend-api/me` ممکن است دیگر `client_id` و `orgs` نداشته باشد؛
نبود این فیلدها به‌تنهایی logout نیست. importer فعلی access token را می‌گیرد،
account check را با Authorization انجام می‌دهد و `id` و `email` پاسخ `/me` را
با هویت `/api/auth/session` تطبیق می‌دهد. درخواست‌های 401/403 همچنان موفق محسوب
نمی‌شوند. اگر هیچ session تأیید نشود، هیچ فایل export قبلی تغییر/حذف نمی‌شود،
sync با خطای روشن متوقف می‌شود و سرور restart نمی‌شود.

برای تست مستقل با پراکسی محلی:

```bash
python3 scripts/import_chatgpt_cookies.py --replace --proxy http://127.0.0.1:10809
```

خطاهای آماده‌سازی مرورگر سرور را نباید به معنی خرابی استخراج محلی دانست.
سلامت Chrome، منابع RAM و شبکهٔ خود سرور نیز باید جدا بررسی شوند.

## ورود دستی با VNC

برای ورود دستی به Chromeهای سرور با Remmina، پروتکل `VNC` و آدرس زیر را وارد کنید:

```text
87.248.139.241:4314
```

رمز VNC از متغیر `VNC_PASSWORD` در `.env` سرور خوانده می‌شود و حداکثر ۸ کاراکتر است.
اگر ترجیح می‌دهید پورت عمومی نباشد،
تونل SSH زیر را از سیستم محلی باز نگه دارید:

```bash
AISTUDIO_SSH_PORT=9011 scripts/open_vnc_tunnel.sh deploy@your-server
```

سپس `http://127.0.0.1:7900/vnc.html` را باز کنید. در حالت تونل، آدرس کلاینت VNC
`127.0.0.1:4314` است.

برای دسترسی محلی به API سرور نیز می‌توانید یک تونل جدا باز کنید:

```bash
ssh -N \
  -L 3346:127.0.0.1:3346 \
  -L 3345:127.0.0.1:3345 \
  deploy@your-server
```

بعد از آن API از روی سیستم محلی با `http://127.0.0.1:3346` قابل استفاده است.

اگر به‌جای انتقال cookie می‌خواهید روی خود سرور login کنید، ابتدا تونل را باز
کنید، در Chrome وارد حساب staging شوید و بعد از آماده‌شدن session، سرویس را
restart نکنید مگر برای reload کردن تنظیمات. volume `browser_profiles` وضعیت Chrome
را نگه می‌دارد.

## بررسی

```bash
curl http://127.0.0.1:3345/health
curl http://127.0.0.1:3346/health
```

برای بررسی remote از طریق SSH tunnel API هم می‌توانید پورت‌های API را tunnel
کنید؛ آن‌ها را روی اینترنت publish نکنید.
