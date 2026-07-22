"""Own the single persistent Chrome session used by the extension."""

import asyncio
import os
from contextlib import suppress
from typing import Any
from urllib.parse import urlparse

from ..broker import TokenBroker
from ..events import emit
from .connection import connect_chrome
from .cookies import apply_and_verify, google_cookie_records, parse_google_cookies
from .cookies import session_fingerprint
from .lifecycle import prime_native_generate
from .navigation import navigate_to_account
from .observer import RpcObserver
from .probe import probe_generate
from .runtime import read_runtime_config
from .transport import normalize_headers, read_transport_profile, rpc_transport_profile

class BrowserSession:
    def __init__(self, broker: TokenBroker, browser_id: str, cdp_url: str):
        self.broker = broker
        self.browser_id = browser_id
        self.cdp_url = cdp_url
        self.playwright: Any = None
        self.browser: Any = None
        self.context: Any = None
        self.page: Any = None
        self.fingerprint: str | None = None
        self.runtime_config: dict[str, Any] | None = None
        self.transport_profile: dict[str, str] | None = None
        self.rpc = RpcObserver()
        self._lock = asyncio.Lock()

    @property
    def ready(self) -> bool:
        return bool(
            self.fingerprint
            and self.runtime_config
            and self.transport_profile
            and rpc_transport_profile(self.rpc.headers).get("x-client-data")
        )

    @property
    def observed_auth_user(self) -> str | None:
        return normalize_headers(self.rpc.headers).get("x-goog-authuser")

    async def prepare(self, cookie_header: str, auth_user: str = "0") -> dict[str, Any]:
        async with self._lock:
            return await self._prepare(cookie_header, str(auth_user))

    async def _prepare(self, cookie_header: str, auth_user: str) -> dict[str, Any]:
        await self._launch()
        cookies = parse_google_cookies(cookie_header)
        if not cookies:
            raise ValueError("No Google cookies were supplied to the container browser")
        fingerprint = session_fingerprint(cookie_header, auth_user)

        if self._same_session(fingerprint):
            await apply_and_verify(self.context, cookies)
            return await self.snapshot()

        await self._reset_page(cookies)
        await navigate_to_account(self.page, auth_user)
        self.runtime_config = await read_runtime_config(self.page)
        self.runtime_config["authUser"] = auth_user
        self.transport_profile = await read_transport_profile(self.page)

        await self._wait_for_extension()
        await prime_native_generate(self.context, self.page)
        await self.rpc.wait_for_identity()
        self.fingerprint = fingerprint
        emit(
            "browser-session-ready",
            browserId=self.browser_id,
            authUser=auth_user,
            cookieCount=len(await self.context.cookies()),
            url=urlparse(self.page.url).path,
        )
        return await self.snapshot()

    async def snapshot(self) -> dict[str, Any]:
        if not self.context or not self.runtime_config or not self.transport_profile:
            raise RuntimeError("Container browser session is not ready")
        return {
            "runtimeConfig": dict(self.runtime_config),
            "transportProfile": {
                **self.transport_profile,
                **rpc_transport_profile(self.rpc.headers),
            },
            "cookieRecords": google_cookie_records(await self.context.cookies()),
        }

    async def probe(self, generate_request: dict[str, Any], token: str) -> dict[str, Any]:
        if not self.page or self.page.is_closed():
            raise RuntimeError("Container AI Studio page is not ready for probe")
        return await probe_generate(
            self.page,
            self.rpc.headers,
            self.runtime_config or {},
            generate_request,
            token,
        )

    async def close(self) -> None:
        browser, playwright = self.browser, self.playwright
        self._clear_connection()
        if browser:
            with suppress(Exception):
                await browser.close()
        if playwright:
            with suppress(Exception):
                await playwright.stop()

    async def _launch(self) -> None:
        if self.context:
            return
        self.playwright, self.browser, self.context = await connect_chrome(
            self.cdp_url,
            self.browser_id,
        )
        self.browser.on("disconnected", self._clear_connection)

    async def _reset_page(self, cookies: list[dict[str, Any]]) -> None:
        pages = self.context.pages
        self.page = pages[0] if pages else await self.context.new_page()
        for extra in pages[1:]:
            with suppress(Exception):
                await extra.close()
        await self.context.clear_cookies()
        await apply_and_verify(self.context, cookies)
        self.rpc.attach(self.page)

    async def _wait_for_extension(self) -> None:
        timeout = int(os.getenv("AISTUDIO_PAGE_READY_TIMEOUT_MS", "60000")) / 1000
        deadline = asyncio.get_running_loop().time() + timeout
        while asyncio.get_running_loop().time() < deadline:
            if self.broker.health(self.browser_id)["connected"]:
                return
            await asyncio.sleep(0.1)
        raise RuntimeError("Container Chrome did not load the staging bridge extension")

    def _same_session(self, fingerprint: str) -> bool:
        return bool(self.page and not self.page.is_closed() and self.fingerprint == fingerprint)

    def _clear_connection(self, *_args: Any) -> None:
        self.browser = None
        self.context = None
        self.page = None
        self.fingerprint = None
