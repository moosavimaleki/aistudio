"""Context manager عمومی برای اندازه‌گیری operation بدون آلودگی منطق اصلی."""

from contextlib import contextmanager
import time

from .store import MetricStore


@contextmanager
def timed(
    store: MetricStore,
    name: str,
    labels: dict[str, object] | None = None,
):
    started = time.perf_counter()
    try:
        yield
    finally:
        store.timing(name, (time.perf_counter() - started) * 1000, labels)
