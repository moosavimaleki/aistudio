package gencontent

const dashboardHTML = `<!doctype html>
<html lang="fa" dir="rtl">
<head>
  <meta charset="utf-8">
  <title>AI Studio Lab Dashboard</title>
  <style>
    body { font: 15px system-ui; margin: 24px; background: #101827; color: #e5e7eb; }
    main { max-width: 1100px; margin: auto; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 14px; }
    .card { padding: 16px; background: #1f2937; border-radius: 10px; }
    table { width: 100%; border-collapse: collapse; }
    td, th { padding: 8px; text-align: right; border-bottom: 1px solid #374151; }
  </style>
</head>
<body>
  <main>
    <h1>AI Studio Lab Dashboard</h1>
    <p id="updated">در حال دریافت داده…</p>
    <div class="grid" id="summary"></div>
    <h2>Browser sessionها</h2>
    <div class="card">
      <table>
        <thead><tr><th>Profile</th><th>Auth user</th><th>Session</th><th>Ready</th><th>Jobهای در انتظار</th><th>Heartbeat</th></tr></thead>
        <tbody id="sessions"></tbody>
      </table>
    </div>
    <h2>HTTP metrics</h2>
    <div class="card"><pre id="metrics"></pre></div>
  </main>
  <script>
    const text = value => document.createTextNode(String(value));
    function cell(row, value) {
      const node = document.createElement('td');
      node.append(text(value));
      row.append(node);
    }
    function card(key, value) {
      const node = document.createElement('div');
      node.className = 'card';
      const title = document.createElement('strong');
      title.append(text(key));
      const body = document.createElement('p');
      body.append(text(value));
      node.append(title, body);
      return node;
    }
    async function refresh() {
      const response = await fetch('/dashboard/data?window=60', { cache: 'no-store' });
      const data = await response.json();
      document.querySelector('#updated').textContent =
        'آخرین بروزرسانی: ' + new Date(data.generatedAt).toLocaleString();
      const summary = document.querySelector('#summary');
      summary.replaceChildren(
        card('Tabهای کل', data.pool.total),
        card('Tab آماده', data.pool.available),
        card('Tab اجاره‌شده', data.pool.leased),
        card('Browser متصل', data.browser.connected ? 'بله' : 'خیر'),
      );
      const sessions = document.querySelector('#sessions');
      sessions.replaceChildren();
      for (const item of data.browser.browsers) {
        const row = document.createElement('tr');
        cell(row, item.browserId);
        cell(row, item.authUser);
        cell(row, item.sessionState);
        cell(row, item.ready ? 'بله' : 'خیر');
        cell(row, item.pendingJobs);
        cell(row, Number(item.heartbeatAgeSeconds).toFixed(1) + 's');
        sessions.append(row);
      }
      if (data.browserError) {
        document.querySelector('#updated').textContent += ' — ' + data.browserError;
      }
      document.querySelector('#metrics').textContent =
        JSON.stringify(data.metrics.aggregate, null, 2);
    }
    refresh().catch(error => {
      document.querySelector('#updated').textContent = 'خطا: ' + error.message;
    });
    setInterval(refresh, 5000);
  </script>
</body>
</html>`
