"""Active virtual-tab manager with transparent session recovery."""

from __future__ import annotations

from typing import Callable

from .config import Settings
from .errors import ClientError
from .http import HttpClient
from .models import GenerateInput, GenerateResult
from .tab import AIStudioTab, TabState, invalidates_tab


ClientState = TabState


class AIStudioClient:
    def __init__(
        self,
        *,
        env_file: str | None = None,
        settings: Settings | None = None,
        http: HttpClient | None = None,
        tab_factory=None,
    ) -> None:
        self.settings = settings or Settings.load(env_file)
        self._state = ClientState.NEW
        self._active_tab: AIStudioTab | None = None
        self._initial_http = http
        self._tab_factory = tab_factory

    @property
    def active_tab(self) -> AIStudioTab | None:
        return self._active_tab

    @property
    def state(self) -> ClientState:
        return self._active_tab.state if self._active_tab else self._state

    @property
    def http(self):
        return self._active_tab.http if self._active_tab else self._initial_http

    @property
    def runtime(self):
        return self._active_tab.runtime if self._active_tab else None

    @property
    def transport_profile(self):
        return self._active_tab.transport_profile if self._active_tab else None

    @property
    def logging_context_extension(self):
        return self._active_tab.logging_context_extension if self._active_tab else None

    @property
    def oauth_access_token(self):
        return self._active_tab.oauth_access_token if self._active_tab else None

    def _new_tab(self) -> AIStudioTab:
        if self._tab_factory:
            return self._tab_factory(self.settings)
        http = self._initial_http
        self._initial_http = None
        return AIStudioTab(self.settings, http=http or HttpClient(self.settings.proxy_url))

    def open_tab(self) -> AIStudioTab:
        self.discard_tab()
        self._state = ClientState.INITIALIZING
        tab = self._new_tab()
        self._active_tab = tab
        try:
            tab.initialize()
            self._state = ClientState.READY
            return tab
        except Exception as error:
            self._state = tab.state
            if invalidates_tab(error):
                self.discard_tab()
                self._state = ClientState.INVALID
            raise

    def discard_tab(self) -> None:
        tab, self._active_tab = self._active_tab, None
        if tab:
            tab.close()
        self._state = ClientState.NEW

    def initialize(self) -> "AIStudioClient":
        if self.state is not ClientState.NEW:
            raise ClientError(f"Cannot initialize client in {self.state} state", phase="CONFIG")
        self.open_tab()
        return self

    def generate(self, input: GenerateInput, *, on_chunk: Callable | None = None) -> GenerateResult:
        if not self._active_tab or self._active_tab.state is not ClientState.READY:
            raise ClientError("Client is not ready", phase="RPC")
        try:
            return self._active_tab.generate(input, on_chunk=on_chunk)
        except Exception as error:
            if not invalidates_tab(error):
                raise

        # runtime/auth tab خراب شده است: حذف کامل، bootstrap تازه و فقط یک retry.
        self.discard_tab()
        replacement = self.open_tab()
        try:
            return replacement.generate(input, on_chunk=on_chunk)
        except Exception as retry_error:
            if invalidates_tab(retry_error):
                self.discard_tab()
                self._state = ClientState.INVALID
            raise

    def close(self) -> None:
        self.discard_tab()
        self._state = ClientState.CLOSED
