"""A reusable virtual AI Studio tab and its initialized runtime state."""

from __future__ import annotations

from enum import StrEnum
import time
from typing import Callable
from uuid import uuid4

from .access_token import generate_access_token
from .auth import AuthContext
from .bootstrap import fetch_runtime_config
from .browser_profile import create_browser_transport_profile
from .config import Settings
from .cookies import CookieJar
from .errors import ClientError
from .headers import compose_makersuite_headers
from .http import HttpClient
from .logging_context import get_logging_context
from .makersuite import build_generate_content_payload, content_binding_digest
from .media import upload_content
from .models import GenerateInput, GenerateResult, RuntimeConfig
from .rpc import rpc_url
from .stream import collect_generate_result
from .token_factory import StagingTokenFactory


MAX_GENERATE_RETRIES = 4
RETRYABLE_GENERATE_STATUSES = {408, 429, 500, 502, 503, 504}


class TabState(StrEnum):
    NEW = "NEW"
    INITIALIZING = "INITIALIZING"
    READY = "READY"
    GENERATING = "GENERATING"
    INVALID = "INVALID"
    FAILED = "FAILED"
    CLOSED = "CLOSED"


def invalidates_tab(error: BaseException) -> bool:
    """Return whether request-scoped runtime/auth state must be discarded."""

    if not isinstance(error, ClientError):
        return False
    if error.status in (401, 403):
        return True
    body = error.response_body or ""
    mismatch_markers = (
        "differs from container Chrome",
        "Container browser session differs",
    )
    return error.phase == "ATTESTATION" and any(marker in body for marker in mismatch_markers)


class AIStudioTab:
    """A virtual tab: initialize once, then generate repeatedly.

    This object is not a real browser tab. It is the Python-side ownership
    boundary for cookie state, runtime keys, Visit ID, OAuth startup state,
    LoggingContext and the Token Factory binding that belong together.
    """

    def __init__(self, settings: Settings, *, http: HttpClient | None = None, tab_id: str | None = None) -> None:
        self.id = tab_id or str(uuid4())
        self.settings = settings
        self.http = http or HttpClient(settings.proxy_url)
        self.state = TabState.NEW
        self.cookies = CookieJar(settings.cookie_header)
        self.auth = AuthContext(settings.origin_url, self.cookies.header)
        self.runtime: RuntimeConfig | None = None
        self.transport_profile: dict[str, str] | None = None
        self.logging_context_extension: str | None = None
        self.oauth_access_token: str | None = None
        self.app_folder_id: str | None = None
        self.token_factory: StagingTokenFactory | None = None
        self.generate_count = 0

    def _sync_session(self) -> None:
        self.auth.set_cookie_header(self.cookies.header)
        if self.token_factory and self.runtime:
            self.token_factory.update_session(self.auth, self.runtime)

    def initialize(self) -> "AIStudioTab":
        if self.state is not TabState.NEW:
            raise ClientError(f"Cannot initialize tab in {self.state} state", phase="CONFIG")
        self.state = TabState.INITIALIZING
        try:
            runtime, factory_profile = fetch_runtime_config(
                self.http,
                self.cookies,
                token_factory_url=self.settings.token_factory_url,
                auth_user=self.settings.auth_user,
                browser_id=self.settings.browser_id,
            )
            self.runtime = runtime
            self._sync_session()
            self.transport_profile = factory_profile or create_browser_transport_profile()
            if not self.settings.token_factory_url or not self.settings.waa_api_key:
                raise ClientError("TOKEN_FACTORY_URL and WAA_API_KEY are required", phase="CONFIG")
            self.token_factory = StagingTokenFactory(
                self.http,
                self.settings.token_factory_url,
                self.settings.waa_api_key,
                self.auth,
                runtime,
                self.settings.browser_id,
            )

            # مراحل startup یک بار برای عمر tab اجرا می‌شوند، نه برای هر prompt.
            first = generate_access_token(self.http, self.cookies, self.auth, runtime, self.transport_profile)
            self._sync_session()
            second = generate_access_token(self.http, self.cookies, self.auth, runtime, self.transport_profile)
            self.oauth_access_token = first or second
            self._sync_session()
            self.logging_context_extension = get_logging_context(
                self.http,
                self.cookies,
                self.auth,
                runtime,
                self.transport_profile,
            )
            self._sync_session()
            self.state = TabState.READY
            return self
        except Exception as error:
            self.state = TabState.INVALID if invalidates_tab(error) else TabState.FAILED
            raise

    def upload_bytes(self, content: bytes, *, mime_type: str, name: str | None = None) -> str:
        """bytes دریافتی API را upload و Drive file id را برمی‌گرداند."""

        if self.state is not TabState.READY:
            raise ClientError("Tab is not ready", phase="UPLOAD")
        file_id, self.app_folder_id = upload_content(
            self,
            content,
            mime_type=mime_type,
            name=name,
            app_folder_id=self.app_folder_id,
        )
        return file_id

    def generate(self, input: GenerateInput, *, on_chunk: Callable | None = None) -> GenerateResult:
        if self.state is not TabState.READY or not self.runtime or not self.transport_profile or not self.token_factory:
            raise ClientError("Tab is not ready", phase="RPC")
        self.state = TabState.GENERATING
        try:
            prepared = GenerateInput(
                model=input.model,
                prompt=input.prompt,
                contents=input.contents,
                history=input.history,
                latest_user_turn=input.latest_user_turn,
                generation_config=input.generation_config,
                safety_settings=input.safety_settings,
                system_instruction=input.system_instruction,
                tools=input.tools,
                continuation_token=input.continuation_token,
                tool_context=input.tool_context,
            )
            payload = build_generate_content_payload(prepared)
            digest = content_binding_digest(payload)

            for attempt in range(MAX_GENERATE_RETRIES + 1):
                try:
                    # retry مرحلهٔ ۷ باید snapshot تازه بگیرد؛ field 5 قبلی هیچ‌وقت
                    # در generateRequest بعدی یا RPC بعدی reuse نمی‌شود.
                    payload[4] = None
                    snapshot_headers = {
                        **self.transport_profile,
                        **compose_makersuite_headers(
                            self.auth,
                            self.cookies.header,
                            self.runtime,
                            logging_context_extension=self.logging_context_extension,
                        ),
                    }
                    snapshot = self.token_factory.snapshot(
                        digest,
                        {
                            "url": rpc_url("GenerateContent"),
                            "method": "POST",
                            "headers": snapshot_headers,
                            "payload": payload.copy(),
                        },
                    )
                    self.cookies.apply_records(snapshot.cookie_records)
                    self._sync_session()

                    runtime = self.runtime
                    if snapshot.runtime_config:
                        runtime = RuntimeConfig(
                            snapshot.runtime_config.get("apiKey", runtime.api_key),
                            snapshot.runtime_config.get("visitId", runtime.visit_id),
                            str(snapshot.runtime_config.get("authUser", runtime.auth_user)),
                            snapshot.runtime_config.get("attestationEnabled", runtime.attestation_enabled),
                        )
                    payload[4] = snapshot.token
                    headers = {
                        **self.transport_profile,
                        **(snapshot.transport_profile or {}),
                        **compose_makersuite_headers(
                            self.auth,
                            self.cookies.header,
                            runtime,
                            logging_context_extension=(
                                snapshot.logging_context_extension or self.logging_context_extension
                            ),
                        ),
                    }
                    response = self.http.request(
                        "POST",
                        rpc_url("GenerateContent"),
                        headers=headers,
                        json=payload,
                        stream=True,
                    )
                    self.cookies.apply_response(response)
                    # GenerateContent می‌تواند cookieهای lifecycle را rotate کند.
                    self._sync_session()
                    if response.ok:
                        result = collect_generate_result(response, on_chunk)
                        self.generate_count += 1
                        self.state = TabState.READY
                        return result

                    body = response.text
                    if response.status_code not in RETRYABLE_GENERATE_STATUSES or attempt == MAX_GENERATE_RETRIES:
                        raise ClientError(
                            f"GenerateContent failed with HTTP {response.status_code}",
                            phase="RPC",
                            status=response.status_code,
                            response_body=body,
                        )
                    response.close()
                except ClientError as error:
                    if invalidates_tab(error):
                        raise
                    retryable = error.retryable or error.status in RETRYABLE_GENERATE_STATUSES
                    if not retryable or attempt == MAX_GENERATE_RETRIES:
                        raise

                time.sleep(0.15 * (attempt + 1))

            raise AssertionError("unreachable generate retry loop")
        except Exception as error:
            self.state = TabState.INVALID if invalidates_tab(error) else TabState.READY
            raise

    def close(self) -> None:
        if self.state is TabState.CLOSED:
            return
        self.http.session.close()
        self.state = TabState.CLOSED
