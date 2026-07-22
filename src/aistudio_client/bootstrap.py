"""Bootstrap runtime config and its explicit Token Factory fallback."""

from __future__ import annotations

import base64
import json
import re

from .browser_profile import BOOTSTRAP_HEADERS
from .cookies import CookieJar
from .errors import ClientError
from .http import HttpClient
from .models import RuntimeConfig
from shared import upstream_value

BOOTSTRAP_URL = upstream_value("aistudio", "bootstrap_url")
ACCOUNT_BOOTSTRAP_URL = upstream_value("aistudio", "account_bootstrap_url")
API_KEY_PROPERTY = upstream_value("runtime", "api_key_property")
VISIT_ID_PROPERTY = upstream_value("runtime", "visit_id_property")
ATTESTATION_PROPERTY = upstream_value("runtime", "attestation_enabled_property")


def extract_runtime_config(document: str, *, auth_user: str = "0", diagnostics: dict | None = None) -> RuntimeConfig:
    api_key = re.search(rf'"{re.escape(API_KEY_PROPERTY)}":"([^"\\]+)"', document)
    raw_visit_id = re.search(rf'"{re.escape(VISIT_ID_PROPERTY)}":"([^"\\]+)"', document)
    if not api_key or not raw_visit_id:
        raise ClientError(
            "Bootstrap response does not contain configured runtime markers",
            phase="CONFIG",
            diagnostics={
                **(diagnostics or {}), "body_bytes": len(document.encode()),
                "has_api_key_marker": f'"{API_KEY_PROPERTY}"' in document,
                "has_visit_id_marker": f'"{VISIT_ID_PROPERTY}"' in document,
                "looks_like_sign_in": bool(re.search(r"accounts\.google\.com|sign in", document, re.I)),
            },
        )
    enabled = re.search(
        rf'"{re.escape(ATTESTATION_PROPERTY)}"\s*:\s*(true|false|"true"|"false")',
        document,
    )
    attestation_enabled = enabled is None or enabled.group(1).strip('"') == "true"
    visit_id = "v1_" + base64.urlsafe_b64encode(raw_visit_id.group(1).encode()).decode().rstrip("=")
    return RuntimeConfig(api_key.group(1), visit_id, str(auth_user), attestation_enabled)


def fetch_runtime_config(
    http: HttpClient,
    cookies: CookieJar,
    *,
    token_factory_url: str | None,
    auth_user: str,
    browser_id: str | None = None,
    url: str | None = None,
) -> tuple[RuntimeConfig, dict[str, str] | None]:
    # Token Factory صاحب Chromeای است که attestation را تولید می‌کند. در این
    # حالت bootstrap همان Chrome باید منبع واحد runtime و fingerprint باشد؛
    # bootstrap مستقیم Python ممکن است 200 شود ولی User-Agent/X-Client-Data
    # متفاوتی در state نگه دارد و درخواست token در 3345 رد شود.
    if token_factory_url:
        factory_url = token_factory_url.rstrip("/").rsplit("/", 1)[0] + "/bootstrap"
        factory_response = http.request(
            "POST",
            factory_url,
            headers={"Content-Type": "application/json"},
            data=json.dumps({
                "cookies": cookies.header,
                "authUser": auth_user,
                **({"browserId": browser_id} if browser_id else {}),
            }),
        )
        if not factory_response.ok:
            raise ClientError(
                f"Browser bootstrap failed with HTTP {factory_response.status_code}",
                phase="CONFIG",
                status=factory_response.status_code,
                response_body=factory_response.text,
            )
        body = factory_response.json()
        config = body.get("runtimeConfig") or {}
        profile = body.get("transportProfile") or {}
        if not config.get("apiKey") or not config.get("visitId"):
            raise ClientError("Browser bootstrap does not contain runtime config", phase="CONFIG")
        if not profile.get("User-Agent") or not profile.get("x-client-data"):
            raise ClientError("Browser bootstrap does not contain shared browser fingerprint", phase="CONFIG")
        cookies.apply_records(body.get("cookieRecords"))
        return RuntimeConfig(
            config["apiKey"],
            config["visitId"],
            str(config.get("authUser", auth_user)),
            config.get("attestationEnabled", True),
        ), profile

    bootstrap_url = url or (
        ACCOUNT_BOOTSTRAP_URL.format(auth_user=auth_user)
        if auth_user != "0" else BOOTSTRAP_URL
    )
    response = http.request("GET", bootstrap_url, headers={"Cookie": cookies.header, **BOOTSTRAP_HEADERS}, retries=4, retryable=True)
    if response.ok:
        cookies.apply_response(response)
        return extract_runtime_config(response.text, auth_user=auth_user, diagnostics={"url": response.url, "redirected": bool(response.history)}), None
    raise ClientError(
        f"Bootstrap failed with HTTP {response.status_code}",
        phase="CONFIG",
        status=response.status_code,
        response_body=response.text,
    )
