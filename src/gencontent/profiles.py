"""Read and select ready Chrome profiles from browser-interface."""

from dataclasses import dataclass
import random
from urllib.parse import urlsplit, urlunsplit

import requests

from aistudio_client.errors import ClientError


@dataclass(frozen=True)
class BrowserProfile:
    slot: int
    browser_id: str
    auth_user: str
    connected: bool
    ready: bool
    warm_error: str | None
    observed_auth_user: str | None = None
    pending_jobs: int = 0
    heartbeat_age_seconds: float | None = None
    session_state: str = "UNKNOWN"
    cookie_count: int = 0
    auth_cookie_count: int = 0
    cookie_revision: int = 0
    cookie_expires_at: int | None = None
    last_cookie_sync_at: int | None = None
    last_ready_at: int | None = None
    cookie_source_updated_at: int | None = None
    cookie_source_current: bool = False


class BrowserProfiles:
    def __init__(self, token_factory_url: str, *, timeout: float = 5.0) -> None:
        self.health_url = _health_url(token_factory_url)
        self.timeout = timeout

    def all(self) -> list[BrowserProfile]:
        try:
            response = requests.get(self.health_url, timeout=self.timeout)
            response.raise_for_status()
            items = response.json().get("browsers", [])
        except Exception as error:
            raise ClientError(
                f"Cannot read browser profiles: {error}",
                phase="CONFIG",
            ) from error
        return [_profile(item, slot) for slot, item in enumerate(items, start=1)]

    def choose(self, excluded: set[str] | None = None) -> BrowserProfile:
        excluded = excluded or set()
        ready = [
            profile
            for profile in self.all()
            if profile.connected
            and profile.ready
            and profile.browser_id not in excluded
        ]
        if not ready:
            raise ClientError("No ready Chrome profile is available", phase="CONFIG")
        return random.SystemRandom().choice(ready)


def _health_url(token_factory_url: str) -> str:
    parts = urlsplit(token_factory_url)
    return urlunsplit((parts.scheme, parts.netloc, "/health", "", ""))


def _profile(item: dict, slot: int) -> BrowserProfile:
    return BrowserProfile(
        slot=slot,
        browser_id=str(item.get("browserId", "")),
        auth_user=str(item.get("authUser", "0")),
        connected=bool(item.get("connected")),
        ready=bool(item.get("ready")),
        warm_error=item.get("warmError"),
        observed_auth_user=item.get("observedAuthUser"),
        pending_jobs=int(item.get("pendingJobs", 0)),
        heartbeat_age_seconds=item.get("heartbeatAgeSeconds"),
        session_state=str(item.get("sessionState", "UNKNOWN")),
        cookie_count=int(item.get("cookieCount", 0)),
        auth_cookie_count=int(item.get("authCookieCount", 0)),
        cookie_revision=int(item.get("cookieRevision", 0)),
        cookie_expires_at=item.get("cookieExpiresAt"),
        last_cookie_sync_at=item.get("lastCookieSyncAt"),
        last_ready_at=item.get("lastReadyAt"),
        cookie_source_updated_at=item.get("cookieSourceUpdatedAt"),
        cookie_source_current=bool(item.get("cookieSourceCurrent")),
    )
