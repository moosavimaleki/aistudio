"""ASGI middleware برای آمار HTTP؛ مستقل از route و business logic."""

from __future__ import annotations

import time
from urllib.parse import unquote
from uuid import uuid4


class MetricsMiddleware:
    def __init__(
        self,
        app,
        *,
        excluded_paths: tuple[str, ...] = (),
        excluded_prefixes: tuple[str, ...] = (),
    ) -> None:
        self.app = app
        self.excluded_paths = excluded_paths
        self.excluded_prefixes = excluded_prefixes

    async def __call__(self, scope, receive, send) -> None:
        if scope["type"] != "http" or self._excluded(scope.get("path", "")):
            await self.app(scope, receive, send)
            return

        store = scope["app"].state.metrics
        path = scope.get("path", "")
        labels = _request_labels(path, scope.get("method", "GET"))
        request_id = uuid4().hex
        status = 500
        started = time.perf_counter()
        store.begin_request(request_id)

        async def observed_send(message):
            nonlocal status
            if message["type"] == "http.response.start":
                status = int(message["status"])
            await send(message)

        try:
            await self.app(scope, receive, observed_send)
        finally:
            elapsed_ms = (time.perf_counter() - started) * 1000
            store.end_request(request_id)
            store.increment("http.response", labels={**labels, "status": status})
            store.timing("http.duration", elapsed_ms, labels)
            if status >= 400:
                store.event(
                    "http",
                    "request-error",
                    route=labels["route"],
                    model=labels.get("model"),
                    status=status,
                )

    def _excluded(self, path: str) -> bool:
        return path in self.excluded_paths or any(
            path.startswith(prefix) for prefix in self.excluded_prefixes
        )


def _request_labels(path: str, method: str) -> dict[str, object]:
    route = _route_name(path)
    labels: dict[str, object] = {"route": route, "method": method}
    marker = "/models/"
    if marker in path:
        model = unquote(path.split(marker, 1)[1]).split(":", 1)[0]
        labels["model"] = model
    return labels


def _route_name(path: str) -> str:
    if path.endswith(":streamGenerateContent"):
        return "streamGenerateContent"
    if path.endswith(":generateContent"):
        return "generateContent"
    if path == "/generate-content":
        return "legacyGenerateContent"
    if path == "/get-token":
        return "getToken"
    if path == "/bootstrap":
        return "bootstrap"
    return path.strip("/") or "root"
