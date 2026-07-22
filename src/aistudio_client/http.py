"""HTTP boundary: retries, explicit proxy routing and never proxy localhost."""

from __future__ import annotations

import time
from urllib.parse import urlsplit

import requests

from .errors import ClientError


def _is_local(url: str) -> bool:
    hostname = urlsplit(url).hostname or ""
    return hostname in {"localhost", "127.0.0.1", "::1"} or "." not in hostname


class HttpClient:
    def __init__(self, proxy_url: str | None, *, timeout: float = 60.0) -> None:
        self.proxy_url = proxy_url
        self.timeout = timeout
        self.session = requests.Session()
        self.session.trust_env = False

    def request(self, method: str, url: str, *, retries: int = 0, retryable: bool = False, **kwargs):
        proxies = None if not self.proxy_url or _is_local(url) else {"http": self.proxy_url, "https": self.proxy_url}
        timeout = kwargs.pop("timeout", self.timeout)
        for attempt in range(retries + 1):
            try:
                response = self.session.request(method, url, proxies=proxies, timeout=timeout, **kwargs)
                if not retryable or response.status_code not in {408, 429, 500, 502, 503, 504} or attempt == retries:
                    return response
            except requests.RequestException as error:
                if attempt == retries:
                    detail = type(error).__name__
                    raise ClientError(
                        f"HTTP transport failed ({detail})",
                        phase="NETWORK",
                        retryable=True,
                    ) from error
            time.sleep(0.15 * (attempt + 1))
        raise AssertionError("unreachable")
