"""پوستهٔ کوچک dashboard؛ داده و assetها endpoint مستقل دارند."""


def render_dashboard() -> str:
    return """<!doctype html>
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
        <p>درخواست‌ها، sessionها، Chrome profileها و virtual tab pool</p>
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

    <section id="summary" class="cards"></section>
    <section id="pipeline" class="pipeline"></section>

    <section class="grid two">
      <article class="panel chart-panel">
        <div class="panel-head"><div><h2>روند درخواست</h2><p>موفق و ناموفق در بازهٔ انتخابی</p></div><div class="legend"><span class="request">کل</span><span class="success">موفق</span><span class="error">خطا</span></div></div>
        <svg id="request-chart" viewBox="0 0 900 260" preserveAspectRatio="none"></svg>
      </article>
      <article class="panel">
        <div class="panel-head"><div><h2>تفکیک خطا</h2><p>بر اساس phase و status</p></div></div>
        <div class="breakdowns"><div><h3>Phase</h3><div id="phase-bars"></div></div><div><h3>Status</h3><div id="status-bars"></div></div></div>
      </article>
    </section>

    <section class="panel">
      <div class="panel-head"><div><h2>Browser & Cookie Sessions</h2><p>فقط metadata امن؛ هیچ cookie یا token نمایش داده نمی‌شود</p></div></div>
      <div class="table-wrap"><table><thead><tr><th>Profile</th><th>Session</th><th>اتصال</th><th>Cookie</th><th>Revision</th><th>Expiry</th><th>Heartbeat</th><th>Warm-up</th></tr></thead><tbody id="profiles"></tbody></table></div>
    </section>

    <section class="grid two">
      <article class="panel">
        <div class="panel-head"><div><h2>مدل‌ها</h2><p>حجم، موفقیت و latency</p></div></div>
        <div class="table-wrap"><table><thead><tr><th>Model</th><th>Request</th><th>Success</th><th>P50</th><th>P95</th><th>Empty</th></tr></thead><tbody id="models"></tbody></table></div>
      </article>
      <article class="panel">
        <div class="panel-head"><div><h2>Tab Pool</h2><p>ظرفیت و lifecycle tabهای مجازی</p></div><span id="pool-badge" class="badge"></span></div>
        <div class="table-wrap"><table><thead><tr><th>Tab</th><th>State</th><th>Profile</th><th>Generate</th><th>Lease</th><th>Age</th><th>Last use</th></tr></thead><tbody id="tabs"></tbody></table></div>
      </article>
    </section>

    <section class="panel">
      <div class="panel-head"><div><h2>آخرین رخدادها</h2><p>لیست محدود Redis با retention هفت‌روزه</p></div></div>
      <div id="events" class="events"></div>
    </section>
    <footer><span id="updated">—</span><span>Auto refresh: 5s</span></footer>
  </main>
  <script src="/dashboard/assets/app.js" defer></script>
</body>
</html>"""
