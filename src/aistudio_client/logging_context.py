"""GetLoggingContext and protobuf-wire encoding of its RPC extension."""

from __future__ import annotations

import base64
import json

from .browser_profile import create_browser_transport_profile
from .cookies import CookieJar
from .headers import compose_makersuite_headers
from .http import HttpClient
from .models import RuntimeConfig
from .auth import AuthContext
from .rpc import unary


def _varint(value: int) -> bytes:
    if value < 0:
        raise ValueError("protobuf varint must be non-negative")
    chunks = bytearray()
    while True:
        byte = value & 0x7F
        value >>= 7
        chunks.append(byte | (0x80 if value else 0))
        if not value:
            return bytes(chunks)


def encode_logging_context_extension(context: list) -> str | None:
    encoded = bytearray()
    for index, value in enumerate(context, start=1):
        if value is None:
            continue
        if isinstance(value, bool):
            value = int(value)
        if isinstance(value, int):
            encoded.extend(_varint(index << 3)); encoded.extend(_varint(value))
        elif isinstance(value, str):
            raw = value.encode(); encoded.extend(_varint((index << 3) | 2)); encoded.extend(_varint(len(raw))); encoded.extend(raw)
        else:
            raise ValueError(f"Unsupported GetLoggingContext field {index}: {type(value).__name__}")
    return base64.b64encode(encoded).decode() if encoded else None


def get_logging_context(http: HttpClient, cookies: CookieJar, auth: AuthContext, runtime: RuntimeConfig, profile: dict[str, str]) -> str | None:
    headers = {**profile, **compose_makersuite_headers(auth, cookies.header, runtime)}
    response = unary(http, cookies, "GetLoggingContext", [0], headers)
    parsed = json.loads(response.text)
    if not isinstance(parsed, list):
        raise ValueError("GetLoggingContext response must be a positional array")
    return encode_logging_context_extension(parsed)
