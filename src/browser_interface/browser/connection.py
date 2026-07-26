"""Connect Playwright to the Chrome process owned by the container."""

import os
from contextlib import suppress
from typing import Any
from urllib.parse import urlparse

from playwright.async_api import async_playwright

from ..events import emit


async def connect_chrome(cdp_url: str, browser_id: str) -> tuple[Any, Any, Any]:
    emit("browser-connect", browserId=browser_id, cdpHost=urlparse(cdp_url).netloc)
    playwright = await async_playwright().start()
    try:
        browser = await playwright.chromium.connect_over_cdp(cdp_url)
        if not browser.contexts:
            raise RuntimeError(
                "Container Chrome does not expose its persistent context over CDP"
            )
        _verify_version(browser.version or "unknown")
        emit(
            "browser-ready",
            browserId=browser_id,
            version=browser.version or "unknown",
        )
        return playwright, browser, browser.contexts[0]
    except Exception:
        with suppress(Exception):
            await playwright.stop()
        raise


def _verify_version(version: str) -> None:
    expected = os.getenv("EXPECTED_BROWSER_MAJOR", "").strip()
    if expected and not version.startswith(f"{expected}."):
        raise RuntimeError(
            f"Container Chrome version {version} does not match required major {expected}"
        )
