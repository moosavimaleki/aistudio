const state = { window: 60, timer: null };
const $ = selector => document.querySelector(selector);
const $$ = selector => [...document.querySelectorAll(selector)];

const escapeHtml = value => String(value ?? "—")
  .replaceAll("&", "&amp;").replaceAll("<", "&lt;")
  .replaceAll(">", "&gt;").replaceAll('"', "&quot;");
const number = value => new Intl.NumberFormat("fa-IR").format(Number(value || 0));
const duration = ms => !ms ? "—" : ms >= 1000 ? `${(ms / 1000).toFixed(1)}s` : `${ms}ms`;
const dateTime = value => value ? new Date(value).toLocaleString("fa-IR") : "—";
const ago = value => {
  if (!value) return "—";
  const seconds = Math.max(0, (Date.now() - Number(value)) / 1000);
  if (seconds < 60) return `${Math.round(seconds)}s`;
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`;
  return `${Math.round(seconds / 3600)}h`;
};
const stateClass = value => ["READY", "available", "success"].includes(value)
  ? "ok" : ["INVALID", "DISCONNECTED", "error"].includes(value) ? "error" : "warn";
const pill = value => `<span class="pill ${stateClass(String(value))}">${escapeHtml(value)}</span>`;

async function refresh() {
  try {
    const response = await fetch(`/dashboard/data?window=${state.window}`, { cache: "no-store" });
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    render(await response.json());
    $("#health").className = "health ok";
    $("#health span").textContent = "سرویس آنلاین";
  } catch (error) {
    $("#health").className = "health error";
    $("#health span").textContent = `خطا: ${error.message}`;
  }
}

function render(data) {
  renderSummary(data);
  renderChart(data.series);
  renderBars("#phase-bars", data.errorPhases);
  renderBars("#status-bars", data.errorStatuses);
  renderProfiles(data.profiles);
  renderModels(data.models);
  renderTabs(data.pool);
  renderEvents(data.events);
  renderPipeline(data.summary);
  $("#updated").textContent = `Updated ${new Date(data.generatedAt).toLocaleTimeString("fa-IR")}`;
}

function renderSummary(data) {
  const s = data.summary;
  const cards = [
    ["درخواست", number(s.requests), `${state.window} دقیقه`, "#65a8ff"],
    ["موفقیت", `${s.successRate}%`, `${number(s.success)} موفق`, "#4bdd9d"],
    ["خطا", number(s.errors), "GenerateContent", "#ff7182"],
    ["در حال اجرا", number(s.inflight), "In flight", "#ffc467"],
    ["Latency P50", duration(s.latencyP50), "تقریبی", "#4bd6c3"],
    ["Latency P95", duration(s.latencyP95), "تقریبی", "#9c8cff"],
    ["Profile آماده", `${data.profiles.filter(p => p.ready).length}/${data.profiles.length}`, "Chrome session", "#4bdd9d"],
    ["Pool", `${data.pool.available}/${data.pool.max}`, `${data.pool.leased} leased`, "#65a8ff"],
  ];
  $("#summary").innerHTML = cards.map(([label, value, hint, color]) =>
    `<div class="card" style="--accent:${color}"><label>${label}</label><strong>${value}</strong><small>${hint}</small></div>`
  ).join("");
}

function renderPipeline(summary) {
  const items = [
    ["RPS", summary.rps],
    ["Token success", number(summary.tokenSuccess)],
    ["Token error", number(summary.tokenErrors)],
    ["Token P95", duration(summary.tokenLatencyP95)],
    ["Cookie rotation", number(summary.cookieRotations)],
    ["Attachment", number(summary.attachments)],
  ];
  $("#pipeline").innerHTML = items.map(([label, value]) =>
    `<span>${label}<b>${value}</b></span>`
  ).join("");
}

function renderChart(series) {
  const svg = $("#request-chart");
  const width = 900, height = 260, pad = 28;
  const maximum = Math.max(1, ...series.flatMap(item => [item.requests, item.success, item.errors]));
  const x = index => pad + index * (width - pad * 2) / Math.max(1, series.length - 1);
  const y = value => height - pad - value * (height - pad * 2) / maximum;
  const points = name => series.map((item, index) => `${x(index)},${y(item[name])}`).join(" ");
  let grid = "";
  for (let index = 0; index <= 4; index++) {
    const lineY = pad + index * (height - pad * 2) / 4;
    const label = Math.round(maximum * (4 - index) / 4);
    grid += `<line class="chart-grid" x1="${pad}" y1="${lineY}" x2="${width - pad}" y2="${lineY}"/><text class="chart-label" x="2" y="${lineY + 3}">${label}</text>`;
  }
  svg.innerHTML = `${grid}<polyline class="chart-line request" points="${points("requests")}"/><polyline class="chart-line success" points="${points("success")}"/><polyline class="chart-line error" points="${points("errors")}"/>`;
}

function renderBars(selector, values) {
  const entries = Object.entries(values || {}).sort((a, b) => b[1] - a[1]);
  const maximum = Math.max(1, ...entries.map(([, value]) => value));
  $(selector).innerHTML = entries.length ? entries.map(([name, value]) =>
    `<div class="bar"><div class="bar-meta"><span>${escapeHtml(name)}</span><b>${number(value)}</b></div><div class="bar-track"><i style="width:${value * 100 / maximum}%"></i></div></div>`
  ).join("") : `<div class="empty">خطایی ثبت نشده است</div>`;
}

function renderProfiles(profiles) {
  $("#profiles").innerHTML = profiles.length ? profiles.map(profile => `
    <tr><td><code>${escapeHtml(profile.browser_id)}</code><br><small>auth ${escapeHtml(profile.auth_user)}</small></td>
    <td>${pill(profile.session_state)}</td><td>${pill(profile.connected ? "READY" : "DISCONNECTED")}</td>
    <td>${profile.cookie_count == null ? "—" : number(profile.cookie_count)}</td>
    <td>${profile.cookie_revision == null ? "—" : number(profile.cookie_revision)}</td>
    <td>${dateTime(profile.cookie_expires_at)}</td>
    <td>${profile.heartbeat_age_seconds == null ? "—" : `${profile.heartbeat_age_seconds.toFixed(1)}s`}</td>
    <td title="${escapeHtml(profile.warm_error)}">${profile.warm_error ? pill("ERROR") : pill(profile.ready ? "READY" : "WARMING")}</td></tr>`).join("") : `<tr><td colspan="8" class="empty">Profileای پیدا نشد</td></tr>`;
}

function renderModels(models) {
  $("#models").innerHTML = models.length ? models.map(model => `
    <tr><td><code>${escapeHtml(model.model)}</code></td><td>${number(model.requests)}</td>
    <td>${model.successRate}%</td><td>${duration(model.p50)}</td><td>${duration(model.p95)}</td><td>${number(model.empty)}</td></tr>`
  ).join("") : `<tr><td colspan="6" class="empty">هنوز درخواستی ثبت نشده است</td></tr>`;
}

function renderTabs(pool) {
  $("#pool-badge").textContent = `${pool.total}/${pool.max}`;
  $("#tabs").innerHTML = pool.tabs?.length ? pool.tabs.map(tab => `
    <tr><td><code>${escapeHtml(tab.tabId).slice(0, 12)}</code></td><td>${pill(tab.status)}</td>
    <td><code>${escapeHtml(tab.browserId)}</code></td><td>${number(tab.generateCount)}</td>
    <td>${number(tab.leaseCount)}</td><td>${ago(tab.createdAt)}</td><td>${ago(tab.lastUsedAt)}</td></tr>`
  ).join("") : `<tr><td colspan="7" class="empty">هنوز Tab آماده‌ای وجود ندارد</td></tr>`;
}

function renderEvents(events) {
  $("#events").innerHTML = events.length ? events.map(event => {
    const details = Object.entries(event).filter(([key]) => !["timestamp", "category", "event"].includes(key))
      .map(([key, value]) => `${key}=${value}`).join(" · ");
    return `<div class="event"><time>${new Date(event.timestamp).toLocaleTimeString("fa-IR")}</time><span class="category">${escapeHtml(event.category)}</span><strong>${escapeHtml(event.event)}</strong><span class="details">${escapeHtml(details)}</span></div>`;
  }).join("") : `<div class="empty">رخدادی ثبت نشده است</div>`;
}

$$('[data-window]').forEach(button => button.addEventListener('click', () => {
  state.window = Number(button.dataset.window);
  $$('[data-window]').forEach(item => item.classList.toggle('active', item === button));
  refresh();
}));

refresh();
state.timer = setInterval(refresh, 5000);
