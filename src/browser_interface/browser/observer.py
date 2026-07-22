"""Observe the real RPC identity emitted by AI Studio in Chrome."""

import asyncio
import os
from typing import Any

from .transport import normalize_headers


class RpcObserver:
    def __init__(self):
        self.headers: dict[str, Any] = {}
        self._page: Any = None

    def attach(self, page: Any) -> None:
        self.headers = {}
        if self._page is page:
            return
        self._page = page
        page.on("request", self._capture)
        page.on("console", self._console)
        page.on("pageerror", self._page_error)

    async def wait_for_identity(self) -> None:
        timeout = int(os.getenv("AISTUDIO_PAGE_READY_TIMEOUT_MS", "60000")) / 1000
        deadline = asyncio.get_running_loop().time() + timeout
        while asyncio.get_running_loop().time() < deadline:
            if normalize_headers(self.headers).get("x-client-data"):
                return
            await asyncio.sleep(0.1)
        raise RuntimeError(
            "Container Chrome did not expose X-Client-Data from an AI Studio RPC"
        )

    async def _capture(self, request: Any) -> None:
        if "MakerSuiteService/" not in request.url:
            return
        try:
            self.headers = await request.all_headers()
        except Exception:
            self.headers = request.headers

    @staticmethod
    def _console(message: Any) -> None:
        if message.type == "error" and "ERR_BLOCKED_BY_CLIENT" not in message.text:
            print(f"browser-console: {message.text}", flush=True)

    @staticmethod
    def _page_error(error: Any) -> None:
        print(f"browser-pageerror: {error}", flush=True)
