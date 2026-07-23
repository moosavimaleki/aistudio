"""خواندن window زمانی محدود از metricهای Redis."""

from __future__ import annotations

from dataclasses import dataclass
import json
import time

from .labels import parse_field
from .store import MetricStore


@dataclass(frozen=True)
class MetricWindow:
    minutes: list[dict[str, float]]
    aggregate: dict[str, float]
    gauges: dict[str, str]
    events: list[dict]
    inflight: int


class MetricsReader:
    def __init__(self, store: MetricStore) -> None:
        self.store = store

    def read(self, window_minutes: int) -> MetricWindow:
        maximum = max(1, self.store.retention_seconds // 60)
        window = min(max(1, int(window_minutes)), maximum)
        current = int(time.time() // 60)
        keys = [
            f"{self.store.prefix}:minute:{minute}"
            for minute in range(current - window + 1, current + 1)
        ]
        pipe = self.store.redis.pipeline(transaction=False)
        for key in keys:
            pipe.hgetall(key)
        pipe.hgetall(f"{self.store.prefix}:gauges")
        pipe.lrange(f"{self.store.prefix}:events", 0, self.store.event_limit - 1)
        results = pipe.execute()

        minute_values = [_decode_hash(value) for value in results[:window]]
        aggregate: dict[str, float] = {}
        for values in minute_values:
            for field, amount in values.items():
                aggregate[field] = aggregate.get(field, 0) + amount
        return MetricWindow(
            minutes=minute_values,
            aggregate=aggregate,
            gauges=_decode_text_hash(results[window]),
            events=_decode_events(results[window + 1]),
            inflight=self.store.inflight(),
        )


def matching(
    values: dict[str, float],
    name: str,
    labels: dict[str, str] | None = None,
) -> float:
    expected = labels or {}
    total = 0.0
    for field, value in values.items():
        metric, dimensions = parse_field(field)
        if metric == name and all(dimensions.get(key) == item for key, item in expected.items()):
            total += value
    return total


def grouped(values: dict[str, float], name: str, label: str) -> dict[str, float]:
    result: dict[str, float] = {}
    for field, value in values.items():
        metric, dimensions = parse_field(field)
        if metric != name or label not in dimensions:
            continue
        key = dimensions[label]
        result[key] = result.get(key, 0) + value
    return result


def _decode_hash(values: dict) -> dict[str, float]:
    return {
        _text(field): float(_text(value))
        for field, value in values.items()
    }


def _decode_text_hash(values: dict) -> dict[str, str]:
    return {_text(field): _text(value) for field, value in values.items()}


def _decode_events(values: list) -> list[dict]:
    events = []
    for value in values:
        try:
            item = json.loads(_text(value))
        except (TypeError, json.JSONDecodeError):
            continue
        if isinstance(item, dict):
            events.append(item)
    return events


def _text(value) -> str:
    return value.decode() if isinstance(value, bytes) else str(value)
