package gencontent

var dashboardHTML = []byte(`<!doctype html>
<html lang="fa" dir="rtl">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>GenContent Operations</title>
  <link rel="stylesheet" href="/dashboard/assets/style.css">
</head>
<body>
  <main>
    <header class="topbar">
      <div>
        <div class="eyebrow">AI STUDIO LAB · OPERATIONS</div>
        <h1>GenContent Control Room</h1>
        <p id="dashboard-subtitle">درخواست‌ها، زمان پردازش و وضعیت کلاینت</p>
      </div>
      <div class="controls">
        <div id="health" class="health"><i></i><span>در حال اتصال</span></div>
        <div class="windows">
          <button data-window="15">۱۵ دقیقه</button>
          <button data-window="60" class="active">۱ ساعت</button>
          <button data-window="1440">۲۴ ساعت</button>
        </div>
      </div>
    </header>

    <section class="panel sync-panel">
      <div class="panel-head"><div><h2>به‌روزرسانی sessionها</h2><p>این دستور را روی سیستم خودت اجرا کن؛ کوکی‌ها منتقل و سرویس با تنظیمات فعلی دوباره اجرا می‌شود.</p></div><button id="copy-sync-command" type="button">کپی دستور</button></div>
      <pre id="sync-command">cd /home/h-mousavi/Projects/Hamed/aistudio-api &amp;&amp; \
scripts/sync_sessions_to_server.sh \
  --ssh-port 22 \
  --import-chatgpt \
  root@152.228.145.18:/opt/services/aistudio-api</pre>
    </section>

    <section class="panel samples-panel">
      <div class="panel-head"><div><h2>نمونه‌های API</h2><p>برای دامنهٔ محافظت‌شده، هدر Authorization را با توکن خودت اضافه کن. برای اجرای محلی، مقدار <code>BASE_URL</code> را روی <code>http://127.0.0.1:3346</code> بگذار.</p></div></div>
      <div class="sample-groups">
        <article class="sample-group">
          <h3>تولید متن · GPT‑6</h3><p>مدل‌ها: <code>chatgpt/gpt-6</code>، <code>chatgpt/gpt-6-thinking-standard</code> و <code>chatgpt/gpt-6-thinking-extended</code></p>
          <div class="sample-card"><div class="sample-card-head"><strong>cURL · chat completions</strong><button type="button" data-copy-target="sample-curl-text">کپی</button></div><pre id="sample-curl-text" class="sample-code">BASE_URL="${OPENAI_BASE_URL:-http://127.0.0.1:3346}"
curl -sS "$BASE_URL/v1/chat/completions" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "chatgpt/gpt-6-thinking-standard",
    "messages": [
      {"role": "system", "content": "Answer briefly."},
      {"role": "user", "content": "What is the capital of Iran?"}
    ]
  }' | jq -r 'if .error then error(.error.message) else .choices[0].message.content end'</pre></div>
          <div class="sample-card"><div class="sample-card-head"><strong>Python · chat completions</strong><button type="button" data-copy-target="sample-python-text">کپی</button></div><pre id="sample-python-text" class="sample-code">import json
import os
import urllib.request

base_url = os.getenv("OPENAI_BASE_URL", "http://127.0.0.1:3346")
payload = {
    "model": "chatgpt/gpt-6-thinking-standard",
    "messages": [{"role": "user", "content": "پایتخت ایران چیست؟"}],
}
request = urllib.request.Request(
    f"{base_url}/v1/chat/completions",
    data=json.dumps(payload).encode(),
    headers={"Content-Type": "application/json"},
    method="POST",
)
with urllib.request.urlopen(request, timeout=240) as response:
    result = json.load(response)
print(result["choices"][0]["message"]["content"])</pre></div>
        </article>
        <article class="sample-group">
          <h3>تولید تصویر · نیازمند حالت مرورگر</h3>
          <div class="sample-card"><div class="sample-card-head"><strong>cURL · image generation</strong><button type="button" data-copy-target="sample-curl-image">کپی</button></div><pre id="sample-curl-image" class="sample-code">set -euo pipefail
BASE_URL="${OPENAI_BASE_URL:-http://127.0.0.1:3346}"
image_b64="$(curl -sS "$BASE_URL/v1/images/generations" \
  -H 'Content-Type: application/json' \
  -d '{"model":"chatgpt-web","prompt":"A minimal blue circle on white","n":1,"response_format":"b64_json"}' \
  | jq -er 'if .error then error(.error.message) else .data[0].b64_json end')"
printf '%s' "$image_b64" | base64 -d &gt; generated.png</pre></div>
          <div class="sample-card"><div class="sample-card-head"><strong>Python · image generation</strong><button type="button" data-copy-target="sample-python-image">کپی</button></div><pre id="sample-python-image" class="sample-code">import base64
import json
import os
import urllib.request

base_url = os.getenv("OPENAI_BASE_URL", "http://127.0.0.1:3346")
payload = {"model": "chatgpt-web", "prompt": "A minimal blue circle on white", "n": 1, "response_format": "b64_json"}
request = urllib.request.Request(
    f"{base_url}/v1/images/generations",
    data=json.dumps(payload).encode(),
    headers={"Content-Type": "application/json"},
    method="POST",
)
with urllib.request.urlopen(request, timeout=420) as response:
    image = json.load(response)["data"][0]["b64_json"]
with open("generated.png", "wb") as output:
    output.write(base64.b64decode(image))</pre></div>
        </article>
        <article class="sample-group">
          <h3>تصویر با عکس مرجع · نیازمند حالت مرورگر</h3>
          <div class="sample-card"><div class="sample-card-head"><strong>cURL · multipart upload</strong><button type="button" data-copy-target="sample-curl-reference">کپی</button></div><pre id="sample-curl-reference" class="sample-code">set -euo pipefail
BASE_URL="${OPENAI_BASE_URL:-http://127.0.0.1:3346}"
image_b64="$(curl -sS "$BASE_URL/v1/images/generations" \
  -F model=chatgpt-web \
  -F 'prompt=این تصاویر را به یک پوستر تبلیغاتی تبدیل کن' \
  -F image=@screen-1.png \
  -F image=@screen-2.png \
  | jq -er 'if .error then error(.error.message) else .data[0].b64_json end')"
printf '%s' "$image_b64" | base64 -d &gt; poster.png</pre></div>
          <div class="sample-card"><div class="sample-card-head"><strong>Python · آمادهٔ repository</strong><button type="button" data-copy-target="sample-python-reference">کپی</button></div><pre id="sample-python-reference" class="sample-code">cd /home/h-mousavi/Projects/Hamed/aistudio-api
OPENAI_BASE_URL=https://aistudio.shekarchi-app.ir \
python examples/chatgpt_image_with_references.py \
  --prompt 'این تصاویر را به یک پوستر تبلیغاتی تبدیل کن' \
  --output poster.png screen-1.png screen-2.png</pre></div>
        </article>
      </div>
    </section>

    <section id="direct-session" class="panel" hidden><div class="panel-head"><div><h2>کلاینت مستقیم ChatGPT</h2><p>ورودی احراز هویت محلی نمایش داده می‌شود؛ اعتبار نشست هنگام ارسال بررسی می‌شود.</p></div><span id="direct-state" class="badge"></span></div><div id="direct-details" class="pipeline"></div></section>

    <section id="summary" class="cards"></section>
    <section id="pipeline" class="pipeline"></section>
    <section class="grid two">
      <article class="panel chart-panel">
        <div class="panel-head"><div><h2>روند درخواست</h2><p>موفق و ناموفق در بازهٔ انتخابی</p></div><div class="legend"><span class="request">کل</span><span class="success">موفق</span><span class="error">خطا</span></div></div>
        <svg id="request-chart" viewBox="0 0 900 260" preserveAspectRatio="none"></svg>
      </article>
      <article class="panel"><div class="panel-head"><div><h2>تفکیک خطا</h2><p>بر اساس phase و status</p></div></div><div class="breakdowns"><div><h3>Phase</h3><div id="phase-bars"></div></div><div><h3>Status</h3><div id="status-bars"></div></div></div></article>
    </section>

    <section class="grid two">
      <article class="panel"><div class="panel-head"><div><h2>توزیع زمان پاسخ</h2><p>تعداد درخواست‌ها در بازه‌های زمانی، از ۱ ثانیه</p></div></div><div id="latency-bars"></div></article>
      <article class="panel"><div class="panel-head"><div><h2>زمان مراحل درخواست</h2><p>میانگین و صدک‌های تقریبی؛ مراحل هم‌پوشانی دارند و مجموع آن‌ها زمان کل نیست.</p></div></div><div class="table-wrap"><table><thead><tr><th>مرحله</th><th>تعداد</th><th>میانگین</th><th>P50</th><th>P95</th></tr></thead><tbody id="timings"></tbody></table></div></article>
    </section>

    <section id="browser-sessions" class="panel" hidden><div class="panel-head"><div><h2>Browser & Cookie Sessions</h2><p>فقط metadata امن؛ هیچ cookie یا token نمایش داده نمی‌شود</p></div></div><div class="table-wrap"><table><thead><tr><th>Profile</th><th>Session</th><th>اتصال</th><th>Cookie</th><th>Revision</th><th>Expiry</th><th>Heartbeat</th><th>Warm-up</th></tr></thead><tbody id="profiles"></tbody></table></div></section>

    <section id="models-layout" class="grid two">
      <article class="panel"><div class="panel-head"><div><h2>مدل‌ها</h2><p>حجم، موفقیت و latency</p></div></div><div class="table-wrap"><table><thead><tr><th>Model</th><th>Request</th><th>Success</th><th>P50</th><th>P95</th><th>Empty</th></tr></thead><tbody id="models"></tbody></table></div></article>
      <article id="browser-pool" class="panel" hidden><div class="panel-head"><div><h2>Tab Pool</h2><p>ظرفیت و lifecycle tabهای مجازی</p></div><span id="pool-badge" class="badge"></span></div><div class="table-wrap"><table><thead><tr><th>Tab</th><th>State</th><th>Profile</th><th>Generate</th><th>Lease</th><th>Age</th><th>Last use</th></tr></thead><tbody id="tabs"></tbody></table></div></article>
    </section>

    <section class="panel"><div class="panel-head"><div><h2>آخرین رخدادها</h2><p>لیست محدود Redis با retention هفت‌روزه</p></div></div><div id="events" class="events"></div></section>
    <footer><span id="updated">—</span><span>Auto refresh: 5s</span></footer>
  </main>
  <script src="/dashboard/assets/app.js" defer></script>
</body>
</html>`)
