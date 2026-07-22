"""Composition of authenticated MakerSuite RPC headers."""

from __future__ import annotations

from .auth import AuthContext, generate_authorization_header
from .models import RuntimeConfig


def compose_makersuite_headers(
    auth: AuthContext,
    cookie_header: str,
    runtime: RuntimeConfig,
    *,
    logging_context_extension: str | None = None,
    referer: str = "https://aistudio.google.com/",
) -> dict[str, str]:
    authorization = generate_authorization_header(auth)
    if not authorization:
        raise ValueError("No SAPISID-family cookie is available for Authorization")
    headers = {
        "Cookie": cookie_header, "Authorization": authorization,
        "X-Goog-Api-Key": runtime.api_key, "X-AIStudio-Visit-Id": runtime.visit_id,
        "X-Goog-AuthUser": runtime.auth_user, "Origin": auth.origin, "Referer": referer,
        "Content-Type": "application/json+protobuf", "X-User-Agent": "grpc-web-javascript/0.1",
    }
    if logging_context_extension:
        headers["X-Goog-Ext-519733851-bin"] = logging_context_extension
    return headers
