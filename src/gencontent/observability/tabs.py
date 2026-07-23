"""Metadata غیرحساس lifecycle مربوط به Tabها."""

from __future__ import annotations

import json
import time


class TabRegistry:
    def __init__(self, redis, prefix: str) -> None:
        self.redis = redis
        self.key = f"{prefix}:tabs"

    def leased(self, tab_id: str, *, is_new: bool) -> None:
        try:
            current = self.get(tab_id)
            now = int(time.time() * 1000)
            current.update({
                "createdAt": current.get("createdAt", now),
                "lastLeaseAt": now,
                "leaseCount": int(current.get("leaseCount", 0)) + 1,
                "status": "leased",
                "isNew": is_new,
            })
            self._save(tab_id, current)
        except Exception:
            pass

    def released(self, tab_id: str, state: dict) -> None:
        try:
            current = self.get(tab_id)
            current.update({
                "lastUsedAt": int(time.time() * 1000),
                "status": "available",
                "browserId": state.get("browserId"),
                "authUser": state.get("authUser"),
                "generateCount": int(state.get("generateCount", 0)),
            })
            self._save(tab_id, current)
        except Exception:
            pass

    def discarded(self, tab_id: str) -> None:
        try:
            self.redis.hdel(self.key, tab_id)
        except Exception:
            pass

    def get(self, tab_id: str) -> dict:
        raw = self.redis.hget(self.key, tab_id)
        if not raw:
            return {}
        try:
            return json.loads(raw.decode() if isinstance(raw, bytes) else raw)
        except (TypeError, json.JSONDecodeError):
            return {}

    def all(self) -> dict[str, dict]:
        result = {}
        for tab_id, raw in self.redis.hgetall(self.key).items():
            key = tab_id.decode() if isinstance(tab_id, bytes) else str(tab_id)
            try:
                result[key] = json.loads(
                    raw.decode() if isinstance(raw, bytes) else raw
                )
            except (TypeError, json.JSONDecodeError):
                continue
        return result

    def _save(self, tab_id: str, value: dict) -> None:
        self.redis.hset(
            self.key,
            tab_id,
            json.dumps(value, ensure_ascii=False, separators=(",", ":")),
        )
