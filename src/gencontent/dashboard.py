"""Render the current pool and Chrome profile state as one small HTML page."""

from datetime import UTC, datetime
from html import escape

from .profiles import BrowserProfile


def render_dashboard(pool: dict, profiles: list[BrowserProfile]) -> str:
    tabs = pool.get("tabs", [])
    profile_rows = "".join(_profile_row(profile, tabs) for profile in profiles)
    tab_rows = "".join(_tab_row(tab) for tab in tabs) or _empty_row(7)
    updated = datetime.now(UTC).strftime("%Y-%m-%d %H:%M:%S UTC")
    return f"""<!doctype html>
<html lang="fa" dir="rtl">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>GenContent Lab</title>
  <style>
    :root{{--bg:#07111f;--panel:#0d1b2d;--line:#20334b;--text:#e8f0fb;--muted:#91a4bd;--ok:#31d0aa;--warn:#ffca6a;--blue:#6ea8fe}}
    *{{box-sizing:border-box}} body{{margin:0;background:radial-gradient(circle at top right,#15345a 0,var(--bg) 42%);color:var(--text);font-family:Vazirmatn,Tahoma,sans-serif;min-height:100vh}}
    main{{max-width:1180px;margin:auto;padding:36px 20px}} header{{display:flex;justify-content:space-between;gap:18px;align-items:end;margin-bottom:24px}}
    h1{{margin:0;font-size:30px}} .subtitle,.updated{{color:var(--muted);font-size:13px}} .cards{{display:grid;grid-template-columns:repeat(4,1fr);gap:14px;margin:20px 0}}
    .card,.panel{{background:rgba(13,27,45,.88);border:1px solid var(--line);border-radius:16px;box-shadow:0 15px 40px #0004}}
    .card{{padding:18px}} .card span{{display:block;color:var(--muted);font-size:13px}} .card strong{{font-size:28px;display:block;margin-top:7px}}
    .panel{{padding:18px;margin-top:16px;overflow:auto}} h2{{font-size:17px;margin:0 0 14px}} table{{width:100%;border-collapse:collapse;white-space:nowrap}}
    th,td{{padding:12px 10px;text-align:right;border-bottom:1px solid var(--line);font-size:13px}} th{{color:var(--muted);font-weight:500}} tr:last-child td{{border:0}}
    .pill{{display:inline-block;padding:4px 9px;border-radius:99px;background:#1a2b40;color:var(--muted)}} .ok{{color:var(--ok);background:#14372f}} .warn{{color:var(--warn);background:#3c301d}}
    code{{direction:ltr;display:inline-block;color:#c9dcf7}} @media(max-width:760px){{.cards{{grid-template-columns:repeat(2,1fr)}} header{{align-items:start;flex-direction:column}}}}
  </style>
</head>
<body><main>
  <header><div><h1>GenContent Lab</h1><div class="subtitle">وضعیت زندهٔ virtual tab pool و Chrome profileها</div></div><div class="updated">آخرین خواندن: {updated}</div></header>
  <section class="cards">
    {_card("کل Tabها", pool.get("total", 0))}
    {_card("آزاد", pool.get("available", 0))}
    {_card("در حال استفاده", pool.get("leased", 0))}
    {_card("Chrome Profile", len(profiles))}
  </section>
  <section class="panel"><h2>Chrome profileها</h2><table><thead><tr><th>Profile</th><th>Auth user</th><th>اتصال</th><th>آماده</th><th>Tab</th><th>پاسخ ۲۰۰</th><th>خطای warm-up</th></tr></thead><tbody>{profile_rows}</tbody></table></section>
  <section class="panel"><h2>Tab pool</h2><table><thead><tr><th>Tab ID</th><th>وضعیت</th><th>Profile</th><th>Auth user</th><th>پاسخ ۲۰۰</th><th>پایان Lease</th></tr></thead><tbody>{tab_rows}</tbody></table></section>
</main></body></html>"""


def _profile_row(profile: BrowserProfile, tabs: list[dict]) -> str:
    owned = [tab for tab in tabs if tab.get("browserId") == profile.browser_id]
    successes = sum(int(tab.get("generateCount", 0)) for tab in owned)
    return "<tr>" + "".join((
        _cell(profile.browser_id, code=True),
        _cell(profile.auth_user),
        _status(profile.connected),
        _status(profile.ready),
        _cell(len(owned)),
        _cell(successes),
        _cell(profile.warm_error or "—"),
    )) + "</tr>"


def _tab_row(tab: dict) -> str:
    expires = tab.get("leaseExpiresAt")
    expiry = datetime.fromtimestamp(expires / 1000, UTC).strftime("%H:%M:%S") if expires else "—"
    return "<tr>" + "".join((
        _cell(tab.get("tabId", "—"), code=True),
        _cell(tab.get("status", "—"), pill=True),
        _cell(tab.get("browserId") or "در حال ساخت", code=True),
        _cell(tab.get("authUser") or "—"),
        _cell(tab.get("generateCount", 0)),
        _cell(expiry),
    )) + "</tr>"


def _card(label: str, value: object) -> str:
    return f'<div class="card"><span>{escape(label)}</span><strong>{escape(str(value))}</strong></div>'


def _status(value: bool) -> str:
    return f'<td><span class="pill {"ok" if value else "warn"}">{"بله" if value else "خیر"}</span></td>'


def _cell(value: object, *, code: bool = False, pill: bool = False) -> str:
    text = escape(str(value))
    if code:
        text = f"<code>{text}</code>"
    if pill:
        text = f'<span class="pill">{text}</span>'
    return f"<td>{text}</td>"


def _empty_row(columns: int) -> str:
    return f'<tr><td colspan="{columns}" style="text-align:center;color:var(--muted)">هنوز Tab آماده‌ای وجود ندارد.</td></tr>'
