import os
import unittest
from uuid import uuid4

from redis import Redis

from gencontent.pool import PoolOverError, RedisTabPool


class RedisTabPoolTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.redis = Redis.from_url(os.getenv("REDIS_URL", "redis://redis:6379/0"))
        try:
            cls.redis.ping()
        except Exception as error:
            raise unittest.SkipTest(f"Redis is unavailable: {error}") from error

    def setUp(self):
        self.pool = RedisTabPool(
            self.redis,
            namespace=f"test:pool:{uuid4().hex}",
            max_size=1,
            wait_seconds=0.05,
        )

    def test_exclusive_lease_timeout_and_reuse(self):
        first = self.pool.acquire()
        self.assertTrue(first.is_new)

        with self.assertRaises(PoolOverError):
            self.pool.acquire()

        self.pool.release(first, {"version": 1, "id": first.tab_id})
        snapshot = self.pool.snapshot()
        self.assertEqual(snapshot["available"], 1)
        self.assertEqual(snapshot["tabs"][0]["tabId"], first.tab_id)
        reused = self.pool.acquire()
        self.assertEqual(reused.tab_id, first.tab_id)
        self.assertEqual(reused.state["version"], 1)

        self.pool.discard(reused)
        self.assertEqual(self.pool.stats()["total"], 0)


if __name__ == "__main__":
    unittest.main()
