"""Own all isolated Chrome processes and browser sessions."""

import asyncio
from dataclasses import dataclass

from ..broker import TokenBroker
from ..config import BrowserConfig, BrowserSpec
from ..errors import BrowserIdentityMismatch, UnknownBrowser
from ..events import emit
from selenium.browser.process import ChromeProcess
from .session import BrowserSession
from .cookies import session_fingerprint


@dataclass
class BrowserSlot:
    spec: BrowserSpec
    process: ChromeProcess
    session: BrowserSession
    warm_error: str | None = None


class BrowserFleet:
    def __init__(self, broker: TokenBroker, config: BrowserConfig):
        self.broker = broker
        self.default_browser_id = config.default_browser_id
        self._slots = {
            spec.browser_id: self._create_slot(spec, config.cdp_base_port + index, index)
            for index, spec in enumerate(config.browsers)
        }

    async def start(self) -> None:
        try:
            await asyncio.gather(*(slot.process.start() for slot in self._slots.values()))
        except Exception:
            await self.close()
            raise

    async def warm(self) -> None:
        await asyncio.gather(*(self._warm_slot(slot) for slot in self._slots.values()))

    async def close(self) -> None:
        await asyncio.gather(
            *(slot.session.close() for slot in self._slots.values()),
            return_exceptions=True,
        )
        await asyncio.gather(
            *(slot.process.stop() for slot in self._slots.values()),
            return_exceptions=True,
        )

    def resolve(self, browser_id: str | None) -> str:
        selected = str(browser_id or self.default_browser_id)
        if selected not in self._slots:
            raise UnknownBrowser(f"Unknown browserId: {selected}")
        return selected

    def session(self, browser_id: str) -> BrowserSession:
        return self._slots[self.resolve(browser_id)].session

    def configured_cookies(self, browser_id: str) -> str | None:
        slot = self._slots[self.resolve(browser_id)]
        return slot.session.current_cookie_header or slot.spec.cookie_header

    def auth_user(self, browser_id: str) -> str:
        return self._slots[self.resolve(browser_id)].spec.auth_user

    def assert_identity(
        self,
        browser_id: str,
        cookie_header: str,
        auth_user: str,
    ) -> None:
        spec = self._slots[self.resolve(browser_id)].spec
        if not spec.cookie_header:
            return
        expected_header = self.configured_cookies(browser_id) or spec.cookie_header
        expected = session_fingerprint(expected_header, spec.auth_user)
        actual = session_fingerprint(cookie_header, auth_user)
        if expected != actual:
            raise BrowserIdentityMismatch(
                f"Cookies do not belong to selected browserId: {browser_id}"
            )

    def status(self) -> list[dict]:
        return [
            _slot_status(browser_id, slot, self.broker.health(browser_id))
            for browser_id, slot in self._slots.items()
        ]

    def _create_slot(self, spec: BrowserSpec, port: int, index: int) -> BrowserSlot:
        process = ChromeProcess(spec, port, index)
        session = BrowserSession(
            self.broker,
            spec.browser_id,
            process.cdp_url,
            spec.cookie_file,
            process.profile_path,
        )
        return BrowserSlot(spec=spec, process=process, session=session)

    async def _warm_slot(self, slot: BrowserSlot) -> None:
        if not slot.spec.cookie_header:
            return
        try:
            await slot.session.prepare(slot.spec.cookie_header, slot.spec.auth_user)
            slot.warm_error = None
            emit("browser-warm-ready", browserId=slot.spec.browser_id)
        except Exception as error:
            slot.warm_error = str(error)
            emit(
                "browser-warm-error",
                browserId=slot.spec.browser_id,
                message=str(error),
            )


def _slot_status(browser_id: str, slot: BrowserSlot, health: dict) -> dict:
    return {
        "browserId": browser_id,
        "authUser": slot.spec.auth_user,
        "observedAuthUser": slot.session.observed_auth_user,
        "ready": slot.session.ready,
        **health,
        "warmError": slot.warm_error,
        "sessionState": _session_state(slot, health["connected"]),
        **slot.session.diagnostics.snapshot(),
        "cookieSourceUpdatedAt": _source_updated_at(slot.spec.cookie_file),
        "cookieSourceCurrent": slot.session.cookie_source_current,
    }


def _session_state(slot: BrowserSlot, connected: bool) -> str:
    if slot.session.ready:
        return "READY"
    if slot.warm_error and (
        "sign-in" in slot.warm_error.lower()
        or "cookie" in slot.warm_error.lower()
    ):
        return "INVALID"
    if connected:
        return "WARMING"
    return "DISCONNECTED"


def _source_updated_at(path) -> int | None:
    if not path or not path.is_file():
        return None
    return int(path.stat().st_mtime * 1000)
