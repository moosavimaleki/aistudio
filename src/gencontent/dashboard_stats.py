"""ساخت view-model داشبورد از metric window و وضعیت زنده."""

from __future__ import annotations

from dataclasses import asdict
from datetime import UTC, datetime
import time

from lab_metrics.query import MetricWindow, grouped, matching
from lab_metrics.store import LATENCY_BUCKETS_MS


def dashboard_snapshot(
    window: MetricWindow,
    window_minutes: int,
    pool: dict,
    profiles: list,
) -> dict:
    total = matching(window.aggregate, "generate.request")
    success = matching(
        window.aggregate,
        "generate.result",
        {"outcome": "success"},
    )
    errors = matching(
        window.aggregate,
        "generate.result",
        {"outcome": "error"},
    )
    return {
        "generatedAt": datetime.now(UTC).isoformat(),
        "windowMinutes": window_minutes,
        "summary": {
            "requests": int(total),
            "success": int(success),
            "errors": int(errors),
            "successRate": _ratio(success, success + errors),
            "inflight": window.inflight,
            "rps": round(total / max(1, window_minutes * 60), 3),
            "latencyP50": _percentile(window.aggregate, "generate.duration", 0.50),
            "latencyP95": _percentile(window.aggregate, "generate.duration", 0.95),
            "tokenSuccess": int(matching(
                window.aggregate, "token.result", {"outcome": "success"}
            )),
            "tokenErrors": int(matching(
                window.aggregate, "token.result", {"outcome": "error"}
            )),
            "tokenLatencyP95": _percentile(
                window.aggregate,
                "http.duration",
                0.95,
                {"route": "getToken"},
            ),
            "cookieRotations": int(matching(window.aggregate, "cookie.rotation")),
            "attachments": int(matching(window.aggregate, "attachment.part")),
        },
        "pool": pool,
        "profiles": [asdict(profile) for profile in profiles],
        "models": _models(window.aggregate),
        "errorPhases": grouped(window.aggregate, "generate.error", "phase"),
        "errorStatuses": grouped(window.aggregate, "generate.error", "status"),
        "series": _series(window.minutes, window_minutes),
        "events": window.events,
    }


def _models(values: dict[str, float]) -> list[dict]:
    requests = grouped(values, "generate.request", "model")
    rows = []
    for model, count in requests.items():
        labels = {"model": model}
        success = matching(
            values,
            "generate.result",
            {**labels, "outcome": "success"},
        )
        errors = matching(
            values,
            "generate.result",
            {**labels, "outcome": "error"},
        )
        rows.append({
            "model": model,
            "requests": int(count),
            "success": int(success),
            "errors": int(errors),
            "successRate": _ratio(success, success + errors),
            "p50": _percentile(values, "generate.duration", 0.50, labels),
            "p95": _percentile(values, "generate.duration", 0.95, labels),
            "empty": int(matching(values, "generate.empty", labels)),
        })
    return sorted(rows, key=lambda item: item["requests"], reverse=True)


def _series(minutes: list[dict[str, float]], window_minutes: int) -> list[dict]:
    current = int(time.time() // 60)
    first = current - window_minutes + 1
    return [
        {
            "timestamp": (first + index) * 60_000,
            "requests": int(matching(values, "generate.request")),
            "success": int(matching(
                values, "generate.result", {"outcome": "success"}
            )),
            "errors": int(matching(
                values, "generate.result", {"outcome": "error"}
            )),
            "p50": _percentile(values, "generate.duration", 0.50),
            "p95": _percentile(values, "generate.duration", 0.95),
        }
        for index, values in enumerate(minutes)
    ]


def _percentile(
    values: dict[str, float],
    name: str,
    ratio: float,
    labels: dict[str, str] | None = None,
) -> int:
    count = matching(values, f"{name}.count", labels)
    if count <= 0:
        return 0
    target = count * ratio
    for boundary in LATENCY_BUCKETS_MS:
        if matching(values, f"{name}.le_{boundary}", labels) >= target:
            return boundary
    return LATENCY_BUCKETS_MS[-1]


def _ratio(value: float, total: float) -> float:
    return round(value * 100 / total, 1) if total else 0.0
