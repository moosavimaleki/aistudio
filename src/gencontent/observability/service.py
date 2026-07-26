"""Decorator object برای آمار GenerateContent بدون تغییر business service."""

from __future__ import annotations

from aistudio_client.errors import ClientError
from lab_metrics import MetricStore, timed
from lab_metrics.labels import dimension


class ObservedGenerateService:
    def __init__(self, service, metrics: MetricStore) -> None:
        self.inner = service
        self.metrics = metrics

    @property
    def settings(self):
        return self.inner.settings

    @property
    def profiles(self):
        return self.inner.profiles

    def generate(self, input):
        model = dimension(input.model.removeprefix("models/"))
        labels = {"model": model}
        self.metrics.increment("generate.request", labels=labels)
        _record_attachments(self.metrics, input.contents, labels)
        try:
            with timed(self.metrics, "generate.duration", labels):
                outcome = self.inner.generate(input)
        except Exception as error:
            phase = dimension(getattr(error, "phase", None) or "unknown")
            status = str(getattr(error, "status", None) or "none")
            self.metrics.increment(
                "generate.result",
                labels={**labels, "outcome": "error", "phase": phase},
            )
            self.metrics.increment(
                "generate.error",
                labels={"phase": phase, "status": status},
            )
            self.metrics.event(
                "generate",
                "generate-error",
                model=model,
                phase=phase,
                status=status,
                errorType=type(error).__name__,
            )
            raise

        self.metrics.increment(
            "generate.result",
            labels={**labels, "outcome": "success"},
        )
        self.metrics.increment(
            "generate.profile",
            labels={"browser": outcome.browser_id, "outcome": "success"},
        )
        self.metrics.increment("generate.chunks", len(outcome.result.chunks), labels)
        if not outcome.result.final_text:
            self.metrics.increment("generate.empty", labels=labels)
        return outcome


def _record_attachments(metrics: MetricStore, contents: list[dict], labels: dict) -> None:
    inline_count = 0
    reused_count = 0
    encoded_bytes = 0
    for content in contents or []:
        for part in content.get("parts", []):
            inline = part.get("inlineData") if isinstance(part, dict) else None
            file_data = part.get("fileData") if isinstance(part, dict) else None
            if inline:
                inline_count += 1
                encoded_bytes += len(str(inline.get("data", "")))
            elif file_data:
                reused_count += 1
    if inline_count:
        metrics.increment("attachment.part", inline_count, {**labels, "kind": "inline"})
        metrics.increment("attachment.base64_bytes", encoded_bytes, labels)
    if reused_count:
        metrics.increment("attachment.part", reused_count, {**labels, "kind": "reused"})
