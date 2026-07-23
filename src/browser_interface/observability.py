"""تبدیل eventهای browser-interface به metricهای امن و کم‌کاردینالیتی."""

from lab_metrics import MetricStore
from lab_metrics.labels import dimension


RECORDED_EVENTS = {
    "browser-warm-ready",
    "browser-warm-error",
    "browser-session-ready",
    "cookies-persisted",
    "token-created",
    "bootstrap-error",
    "token-error",
    "same-browser-provider-probe",
}


class BrowserEventMetrics:
    def __init__(self, metrics: MetricStore) -> None:
        self.metrics = metrics

    def __call__(self, event: str, fields: dict) -> None:
        if event not in RECORDED_EVENTS:
            return
        browser = dimension(fields.get("browserId") or "unknown")
        self.metrics.increment(
            "browser.event",
            labels={"event": event, "browser": browser},
        )
        if event == "token-created":
            self.metrics.increment("token.result", labels={"outcome": "success", "browser": browser})
        elif event == "token-error":
            self.metrics.increment("token.result", labels={"outcome": "error", "browser": browser})
        elif event == "cookies-persisted":
            self.metrics.increment("cookie.persist", labels={"browser": browser})
            if fields.get("cookieChanged"):
                self.metrics.increment("cookie.rotation", labels={"browser": browser})
        elif event == "browser-warm-error":
            self.metrics.increment("browser.warm", labels={"outcome": "error", "browser": browser})
        elif event == "browser-warm-ready":
            self.metrics.increment("browser.warm", labels={"outcome": "success", "browser": browser})

        self.metrics.event(
            "browser",
            event,
            browserId=browser,
            authUser=fields.get("authUser"),
            cookieCount=fields.get("cookieCount"),
            cookieRevision=fields.get("cookieRevision"),
            cookieChanged=fields.get("cookieChanged"),
            status=fields.get("status"),
            code=fields.get("code"),
        )
