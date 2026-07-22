"""Serialize READY virtual tabs without persisting worker-local HTTP sessions."""

from __future__ import annotations

from typing import Any

from .auth import AuthContext
from .config import Settings
from .cookies import CookieJar
from .errors import ClientError
from .http import HttpClient
from .models import RuntimeConfig
from .tab import AIStudioTab, TabState
from .token_factory import StagingTokenFactory


def dump_tab(tab: AIStudioTab) -> dict[str, Any]:
    if tab.state is not TabState.READY or not tab.runtime or not tab.transport_profile:
        raise ClientError("Only a ready tab can be persisted", phase="CONFIG")
    return {
        "version": 2,
        "id": tab.id,
        "browserId": tab.settings.browser_id,
        "authUser": tab.settings.auth_user,
        "cookieHeader": tab.cookies.header,
        "runtime": {
            "apiKey": tab.runtime.api_key,
            "visitId": tab.runtime.visit_id,
            "authUser": tab.runtime.auth_user,
            "attestationEnabled": tab.runtime.attestation_enabled,
        },
        "transportProfile": tab.transport_profile,
        "loggingContextExtension": tab.logging_context_extension,
        "oauthAccessToken": tab.oauth_access_token,
        "appFolderId": getattr(tab, "app_folder_id", None),
        "generateCount": tab.generate_count,
    }


def restore_tab(settings: Settings, data: dict[str, Any], *, http: HttpClient | None = None) -> AIStudioTab:
    runtime_data = data.get("runtime") or {}
    if data.get("version") != 2 or not _is_complete(data, runtime_data):
        raise ClientError("Persisted tab state is incomplete or unsupported", phase="CONFIG")

    tab = AIStudioTab(settings, http=http, tab_id=str(data["id"]))
    tab.cookies = CookieJar(str(data["cookieHeader"]))
    tab.auth = AuthContext(settings.origin_url, tab.cookies.header)
    tab.runtime = RuntimeConfig(
        api_key=str(runtime_data["apiKey"]),
        visit_id=str(runtime_data["visitId"]),
        auth_user=str(runtime_data.get("authUser", settings.auth_user)),
        attestation_enabled=bool(runtime_data.get("attestationEnabled", True)),
    )
    tab.transport_profile = {str(key): str(value) for key, value in data["transportProfile"].items()}
    tab.logging_context_extension = data.get("loggingContextExtension")
    tab.oauth_access_token = data.get("oauthAccessToken")
    tab.app_folder_id = data.get("appFolderId")
    tab.generate_count = int(data.get("generateCount", 0))
    tab.token_factory = _token_factory(tab, settings)
    tab.state = TabState.READY
    return tab


def _is_complete(data: dict, runtime: dict) -> bool:
    return bool(
        data.get("id")
        and data.get("cookieHeader")
        and data.get("transportProfile")
        and runtime.get("apiKey")
        and runtime.get("visitId")
    )


def _token_factory(tab: AIStudioTab, settings: Settings) -> StagingTokenFactory:
    if not settings.token_factory_url or not settings.waa_api_key or not tab.runtime:
        raise ClientError(
            "TOKEN_FACTORY_URL and opaque.waa_api_key are required",
            phase="CONFIG",
        )
    return StagingTokenFactory(
        tab.http,
        settings.token_factory_url,
        settings.waa_api_key,
        tab.auth,
        tab.runtime,
        tab.settings.browser_id,
    )
