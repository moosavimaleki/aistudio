"""وضعیت غیرحساس cookie/session برای health و dashboard."""

from __future__ import annotations

import hashlib
import time
from typing import Any

from .cookies import AUTH_COOKIE_NAMES


class SessionDiagnostics:
    def __init__(self) -> None:
        self.cookie_count = 0
        self.auth_cookie_count = 0
        self.cookie_revision = 0
        self.cookie_expires_at: int | None = None
        self.last_cookie_sync_at: int | None = None
        self.last_ready_at: int | None = None
        self._signature: str | None = None

    def record_snapshot(self, cookies: list[dict[str, Any]]) -> bool:
        signature = _signature(cookies)
        changed = self._signature is not None and signature != self._signature
        if signature != self._signature:
            self.cookie_revision += 1
            self._signature = signature
        self.cookie_count = len(cookies)
        auth = [cookie for cookie in cookies if cookie.get("name") in AUTH_COOKIE_NAMES]
        self.auth_cookie_count = len(auth)
        self.cookie_expires_at = _nearest_expiry(auth)
        self.last_cookie_sync_at = int(time.time() * 1000)
        return changed

    def record_ready(self) -> None:
        self.last_ready_at = int(time.time() * 1000)

    def snapshot(self) -> dict[str, object]:
        return {
            "cookieCount": self.cookie_count,
            "authCookieCount": self.auth_cookie_count,
            "cookieRevision": self.cookie_revision,
            "cookieExpiresAt": self.cookie_expires_at,
            "lastCookieSyncAt": self.last_cookie_sync_at,
            "lastReadyAt": self.last_ready_at,
        }


def _signature(cookies: list[dict[str, Any]]) -> str:
    values = "\n".join(
        f"{cookie.get('name', '')}={cookie.get('value', '')}"
        for cookie in sorted(cookies, key=lambda item: str(item.get("name", "")))
    )
    return hashlib.sha256(values.encode()).hexdigest()


def _nearest_expiry(cookies: list[dict[str, Any]]) -> int | None:
    now = time.time()
    expiries = [
        float(cookie["expires"])
        for cookie in cookies
        if isinstance(cookie.get("expires"), (int, float))
        and float(cookie["expires"]) > now
    ]
    return int(min(expiries) * 1000) if expiries else None
