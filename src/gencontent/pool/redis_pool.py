"""Atomic Redis adapter for leasing initialized virtual tabs."""

from __future__ import annotations

import json
from pathlib import Path
import time
from uuid import uuid4

from redis import Redis

from .lease import PoolOverError, TabLease


LUA_DIR = Path(__file__).with_name("lua")


class RedisTabPool:
    def __init__(
        self,
        redis: Redis,
        *,
        max_size: int = 100,
        wait_seconds: float = 5.0,
        lease_seconds: float = 600.0,
        namespace: str = "gencontent:tabs",
    ) -> None:
        if max_size < 1 or wait_seconds < 0 or lease_seconds <= 0:
            raise ValueError("Invalid tab pool settings")
        self.redis = redis
        self.max_size = max_size
        self.wait_seconds = wait_seconds
        self.lease_seconds = lease_seconds
        self.keys = _keys(namespace)
        self._acquire = _script(redis, "acquire.lua")
        self._release = _script(redis, "release.lua")
        self._discard = _script(redis, "discard.lua")

    def acquire(self) -> TabLease:
        deadline = time.monotonic() + self.wait_seconds
        while True:
            lease = self._try_acquire()
            if lease:
                return lease
            if time.monotonic() >= deadline:
                raise PoolOverError(f"Tab pool is full after waiting {self.wait_seconds:g} seconds")
            time.sleep(min(0.1, max(0, deadline - time.monotonic())))

    def _try_acquire(self) -> TabLease | None:
        now_ms = int(time.time() * 1000)
        lease_token = uuid4().hex
        result = self._acquire(
            keys=[
                self.keys["total"],
                self.keys["available"],
                self.keys["states"],
                self.keys["leases"],
                self.keys["expirations"],
            ],
            args=[
                now_ms,
                now_ms + int(self.lease_seconds * 1000),
                self.max_size,
                lease_token,
                str(uuid4()),
            ],
        )
        if not result:
            return None
        tab_id, raw_state, is_new = (_text(value) for value in result)
        state = None if is_new == "1" else json.loads(raw_state)
        return TabLease(tab_id, lease_token, state)

    def release(self, lease: TabLease, state: dict) -> None:
        encoded = json.dumps(state, ensure_ascii=False, separators=(",", ":"))
        released = self._release(
            keys=[
                self.keys["available"],
                self.keys["states"],
                self.keys["leases"],
                self.keys["expirations"],
            ],
            args=[lease.tab_id, lease.token, encoded],
        )
        if not released:
            raise RuntimeError(f"Lease for tab {lease.tab_id} expired before release")

    def discard(self, lease: TabLease) -> None:
        self._discard(
            keys=[
                self.keys["available"],
                self.keys["states"],
                self.keys["leases"],
                self.keys["expirations"],
                self.keys["total"],
            ],
            args=[lease.tab_id, lease.token],
        )

    def stats(self) -> dict[str, int]:
        pipe = self.redis.pipeline(transaction=False)
        pipe.get(self.keys["total"])
        pipe.llen(self.keys["available"])
        pipe.hlen(self.keys["leases"])
        total, available, leased = pipe.execute()
        return {
            "total": int(total or 0),
            "available": int(available),
            "leased": int(leased),
            "max": self.max_size,
        }

    def snapshot(self) -> dict:
        pipe = self.redis.pipeline(transaction=False)
        pipe.hgetall(self.keys["states"])
        pipe.hkeys(self.keys["leases"])
        pipe.zrange(self.keys["expirations"], 0, -1, withscores=True)
        states, leased_ids, expirations = pipe.execute()
        decoded_states = {_text(key): json.loads(_text(value)) for key, value in states.items()}
        leased = {_text(value) for value in leased_ids}
        expiry = {_text(tab_id): int(score) for tab_id, score in expirations}
        tab_ids = sorted(set(decoded_states) | leased)
        return {
            **self.stats(),
            "tabs": [
                _tab_summary(
                    tab_id,
                    decoded_states.get(tab_id, {}),
                    tab_id in leased,
                    expiry.get(tab_id),
                )
                for tab_id in tab_ids
            ],
        }


def _keys(namespace: str) -> dict[str, str]:
    return {name: f"{namespace}:{name}" for name in ("total", "available", "states", "leases", "expirations")}


def _script(redis: Redis, name: str):
    return redis.register_script((LUA_DIR / name).read_text(encoding="utf-8"))


def _text(value) -> str:
    return value.decode("utf-8") if isinstance(value, bytes) else str(value)


def _tab_summary(tab_id: str, state: dict, leased: bool, expires_at: int | None) -> dict:
    return {
        "tabId": tab_id,
        "status": "leased" if leased else "available",
        "browserId": state.get("browserId"),
        "authUser": state.get("authUser"),
        "generateCount": int(state.get("generateCount", 0)),
        "leaseExpiresAt": expires_at,
    }
