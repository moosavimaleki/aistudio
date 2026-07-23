"""نوشتن metricهای دقیقه‌ای، gauge و event در Redis."""

from __future__ import annotations

import json
import time
from typing import Any

from redis import Redis

from .labels import metric_field


LATENCY_BUCKETS_MS = (100, 250, 500, 1000, 2500, 5000, 10000, 30000, 60000)


class MetricStore:
    def __init__(
        self,
        redis: Redis,
        *,
        prefix: str = "lab:metrics:v1",
        retention_seconds: int = 172800,
        event_retention_seconds: int = 604800,
        event_limit: int = 200,
    ) -> None:
        self.redis = redis
        self.prefix = prefix
        self.retention_seconds = max(3600, retention_seconds)
        self.event_retention_seconds = max(3600, event_retention_seconds)
        self.event_limit = max(20, event_limit)

    def increment(
        self,
        name: str,
        amount: float = 1,
        labels: dict[str, object] | None = None,
        *,
        timestamp: float | None = None,
    ) -> None:
        key = self.minute_key(timestamp)
        pipe = self.redis.pipeline(transaction=False)
        pipe.hincrbyfloat(key, metric_field(name, labels), amount)
        pipe.expire(key, self.retention_seconds)
        _execute(pipe)

    def timing(
        self,
        name: str,
        elapsed_ms: float,
        labels: dict[str, object] | None = None,
        *,
        timestamp: float | None = None,
    ) -> None:
        key = self.minute_key(timestamp)
        pipe = self.redis.pipeline(transaction=False)
        pipe.hincrbyfloat(key, metric_field(f"{name}.count", labels), 1)
        pipe.hincrbyfloat(key, metric_field(f"{name}.sum_ms", labels), elapsed_ms)
        for boundary in LATENCY_BUCKETS_MS:
            if elapsed_ms <= boundary:
                pipe.hincrbyfloat(
                    key,
                    metric_field(f"{name}.le_{boundary}", labels),
                    1,
                )
        pipe.hincrbyfloat(key, metric_field(f"{name}.le_inf", labels), 1)
        pipe.expire(key, self.retention_seconds)
        _execute(pipe)

    def gauge(
        self,
        name: str,
        value: object,
        labels: dict[str, object] | None = None,
    ) -> None:
        key = f"{self.prefix}:gauges"
        pipe = self.redis.pipeline(transaction=False)
        pipe.hset(key, metric_field(name, labels), str(value))
        pipe.expire(key, self.retention_seconds)
        _execute(pipe)

    def event(self, category: str, event: str, **fields: Any) -> None:
        payload = {
            "timestamp": int(time.time() * 1000),
            "category": category,
            "event": event,
            **{key: value for key, value in fields.items() if value is not None},
        }
        key = f"{self.prefix}:events"
        pipe = self.redis.pipeline(transaction=False)
        pipe.lpush(key, json.dumps(payload, ensure_ascii=False, separators=(",", ":")))
        pipe.ltrim(key, 0, self.event_limit - 1)
        pipe.expire(key, self.event_retention_seconds)
        _execute(pipe)

    def begin_request(self, request_id: str, *, timeout_seconds: int = 300) -> None:
        key = f"{self.prefix}:inflight"
        pipe = self.redis.pipeline(transaction=False)
        pipe.zadd(key, {request_id: time.time() + timeout_seconds})
        pipe.expire(key, timeout_seconds * 2)
        _execute(pipe)

    def end_request(self, request_id: str) -> None:
        try:
            self.redis.zrem(f"{self.prefix}:inflight", request_id)
        except Exception:
            pass

    def inflight(self) -> int:
        key = f"{self.prefix}:inflight"
        pipe = self.redis.pipeline(transaction=False)
        pipe.zremrangebyscore(key, "-inf", time.time())
        pipe.zcard(key)
        try:
            _, count = pipe.execute()
            return int(count)
        except Exception:
            return 0

    def minute_key(self, timestamp: float | None = None) -> str:
        minute = int((timestamp or time.time()) // 60)
        return f"{self.prefix}:minute:{minute}"


def _execute(pipe) -> None:
    try:
        pipe.execute()
    except Exception:
        # metric هیچ‌وقت نباید business request را fail کند.
        pass
