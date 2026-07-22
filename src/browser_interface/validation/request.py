"""Validate that an attestation request belongs to its GenerateContent body."""

import hashlib
import re
from typing import Any


def normalize_headers(headers: Any) -> dict[str, str]:
    if not isinstance(headers, dict):
        return {}
    return {str(name).lower(): str(value) for name, value in headers.items()}


def compute_content_digest(payload: Any) -> str:
    if not isinstance(payload, list) or len(payload) < 2 or not isinstance(payload[1], list):
        raise ValueError("GenerateContent context has no contents field")
    digest_parts: list[str] = []
    for turn in payload[1]:
        parts = turn[0] if isinstance(turn, list) and turn and isinstance(turn[0], list) else []
        for part in parts:
            digest_parts.append(_part_digest_value(part))
    content = " ".join(digest_parts).encode()
    return hashlib.sha256(content).hexdigest()


def _part_digest_value(part: Any) -> str:
    if not isinstance(part, list):
        return ""
    if len(part) > 1 and isinstance(part[1], str):
        return part[1]
    if len(part) > 5 and isinstance(part[5], list) and part[5]:
        file_id = part[5][0]
        return file_id if isinstance(file_id, str) else ""
    return ""


def validate_token_request(body: Any) -> tuple[dict[str, str], str]:
    if not isinstance(body, dict):
        raise ValueError("request body is required")
    digest = body.get("digest", "")
    cookies = body.get("cookies")
    authorization = body.get("authorization")
    waa_api_key = body.get("waaApiKey")
    auth_user = str(body.get("authUser", "0"))
    generate = body.get("generateRequest")

    if not re.fullmatch(r"[a-f0-9]{64}", str(digest)):
        raise ValueError("digest must be lowercase SHA-256 hex")
    if not isinstance(cookies, str) or not cookies.strip():
        raise ValueError("cookies are required")
    if not isinstance(authorization, str) or not authorization:
        raise ValueError("authorization is required")
    if not isinstance(waa_api_key, str) or not waa_api_key:
        raise ValueError("waaApiKey is required")
    if not _is_generate_request(generate):
        raise ValueError("pending GenerateContent request context is required")
    if compute_content_digest(generate["payload"]) != digest:
        raise ValueError("GenerateContent body does not match digest")

    headers = normalize_headers(generate.get("headers"))
    required = (
        "authorization", "cookie", "origin", "user-agent", "x-client-data",
        "x-goog-api-key", "x-goog-authuser",
    )
    for name in required:
        if not headers.get(name):
            raise ValueError(f"GenerateContent context is missing {name}")
    if headers["origin"] != "https://aistudio.google.com":
        raise ValueError("GenerateContent origin does not match AI Studio")
    if headers["cookie"] != cookies:
        raise ValueError("GenerateContent cookie context differs from Token Factory cookies")
    if headers["x-goog-authuser"] != auth_user:
        raise ValueError("GenerateContent auth-user differs from Token Factory auth-user")
    return headers, auth_user


def _is_generate_request(value: Any) -> bool:
    return (
        isinstance(value, dict)
        and value.get("method") == "POST"
        and isinstance(value.get("url"), str)
        and "MakerSuiteService/GenerateContent" in value["url"]
        and isinstance(value.get("payload"), list)
    )
