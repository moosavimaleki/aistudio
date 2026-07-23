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
    )
