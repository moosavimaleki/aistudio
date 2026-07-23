"""ساخت MetricStore از environment مشترک سرویس‌ها."""

import os

from redis import Redis

from .store import MetricStore


def create_metric_store(redis: Redis | None = None) -> MetricStore:
    connection = redis or Redis.from_url(
        os.getenv("REDIS_URL", "redis://redis:6379/0"),
        decode_responses=False,
    )
    return MetricStore(
        connection,
        prefix=os.getenv("METRICS_NAMESPACE", "lab:metrics:v1"),
        retention_seconds=int(os.getenv("METRICS_RETENTION_SECONDS", "172800")),
        event_retention_seconds=int(
            os.getenv("METRICS_EVENT_RETENTION_SECONDS", "604800")
        ),
        event_limit=int(os.getenv("METRICS_EVENT_LIMIT", "200")),
    )
