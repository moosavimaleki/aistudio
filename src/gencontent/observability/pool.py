"""Decorator مستقل برای observability مربوط به RedisTabPool."""

from __future__ import annotations

import time

from gencontent.pool import PoolOverError
from lab_metrics import MetricStore

from .tabs import TabRegistry


class ObservedTabPool:
    def __init__(self, pool, metrics: MetricStore) -> None:
        self.inner = pool
        self.metrics = metrics
        self.registry = TabRegistry(pool.redis, metrics.prefix)

    @property
    def redis(self):
        return self.inner.redis

    def acquire(self):
        started = time.perf_counter()
        try:
            lease = self.inner.acquire()
        except PoolOverError:
            self.metrics.increment("pool.over")
            self.metrics.event("pool", "pool-over")
            raise
        finally:
            self.metrics.timing(
                "pool.wait",
                (time.perf_counter() - started) * 1000,
            )
        kind = "new" if lease.is_new else "reused"
        self.metrics.increment("pool.acquire", labels={"kind": kind})
        self.registry.leased(lease.tab_id, is_new=lease.is_new)
        return lease

    def release(self, lease, state: dict) -> None:
        self.inner.release(lease, state)
        self.registry.released(lease.tab_id, state)
        self.metrics.increment("pool.release")

    def discard(self, lease) -> None:
        self.inner.discard(lease)
        self.registry.discarded(lease.tab_id)
        self.metrics.increment("pool.discard")
        self.metrics.event("tab", "tab-discarded", tabId=lease.tab_id)

    def stats(self) -> dict:
        return self.inner.stats()

    def snapshot(self) -> dict:
        snapshot = self.inner.snapshot()
        metadata = self.registry.all()
        snapshot["tabs"] = [
            {**metadata.get(tab["tabId"], {}), **tab}
            for tab in snapshot.get("tabs", [])
        ]
        return snapshot
