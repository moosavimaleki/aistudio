"""Startup GenerateAccessToken RPC. Its token is lifecycle state, not auth."""

from __future__ import annotations

import json

from .auth import AuthContext
from .cookies import CookieJar
from .headers import compose_makersuite_headers
from .http import HttpClient
from .models import RuntimeConfig
from .rpc import unary


def generate_access_token(http: HttpClient, cookies: CookieJar, auth: AuthContext, runtime: RuntimeConfig, profile: dict[str, str], logging_extension: str | None = None) -> str | None:
    headers = {**profile, **compose_makersuite_headers(auth, cookies.header, runtime, logging_context_extension=logging_extension)}
    response = unary(http, cookies, "GenerateAccessToken", ["users/me"], headers)
    try:
        parsed = json.loads(response.text)
    except json.JSONDecodeError:
        return None
    return parsed[0] if isinstance(parsed, list) and parsed and isinstance(parsed[0], str) else None
