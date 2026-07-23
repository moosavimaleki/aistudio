import os
import unittest
from uuid import uuid4

from redis import Redis

from lab_metrics.labels import metric_field
from lab_metrics.query import MetricsReader, matching
from lab_metrics.store import MetricStore


class MetricStoreTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.redis = Redis.from_url(
            os.getenv("REDIS_URL", "redis://redis:6379/0"),
            decode_responses=False,
        )
        try:
            cls.redis.ping()
        except Exception as error:
            raise unittest.SkipTest(f"Redis is unavailable: {error}") from error

    def setUp(self):
        self.prefix = f"test:metrics:{uuid4().hex}"
        self.store = MetricStore(
            self.redis,
            prefix=self.prefix,
            retention_seconds=3600,
            event_limit=2,
        )

    def tearDown(self):
        keys = list(self.redis.scan_iter(f"{self.prefix}:*"))
        if keys:
            self.redis.delete(*keys)

    def test_counter_timing_event_and_inflight_are_bounded(self):
        labels = {"model": "gemini-test"}
        self.store.increment("generate.request", labels=labels)
        self.store.timing("generate.duration", 240, labels)
        for index in range(21):
            self.store.event("generate", f"event-{index}")
        self.store.begin_request("request-1")

        window = MetricsReader(self.store).read(2)

        self.assertEqual(matching(window.aggregate, "generate.request"), 1)
        self.assertEqual(
            matching(window.aggregate, "generate.duration.le_250", labels),
            1,
        )
        self.assertEqual(window.inflight, 1)
        self.assertEqual(len(window.events), 20)
        self.assertEqual(window.events[0]["event"], "event-20")
        self.assertEqual(window.events[-1]["event"], "event-1")
        self.assertGreater(self.redis.ttl(self.store.minute_key()), 0)

        self.store.end_request("request-1")
        self.assertEqual(self.store.inflight(), 0)

    def test_metric_labels_do_not_store_spaces_or_unbounded_values(self):
        field = metric_field("generate request", {"model": "model with spaces"})

        self.assertNotIn(" ", field)
        self.assertLessEqual(len(field), 180)


if __name__ == "__main__":
    unittest.main()
