"""Environment-backed construction of the shared tab pool."""

import os

from redis import Redis

from .pool import RedisTabPool


def create_pool() -> RedisTabPool:
    redis = Redis.from_url(os.getenv("REDIS_URL", "redis://redis:6379/0"), decode_responses=False)
    redis.ping()
    return RedisTabPool(
        redis,
        max_size=int(os.getenv("TAB_POOL_MAX", "100")),
        wait_seconds=float(os.getenv("TAB_POOL_WAIT_SECONDS", "5")),
        lease_seconds=float(os.getenv("TAB_POOL_LEASE_SECONDS", "600")),
        namespace=os.getenv("TAB_POOL_NAMESPACE", "gencontent:tabs:v2"),
    )
