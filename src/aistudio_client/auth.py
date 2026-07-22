"""The SAPISID-family Authorization header, independent of any browser API."""

from __future__ import annotations

import hashlib
import time
from dataclasses import dataclass
from collections.abc import Callable, Sequence
from urllib.parse import urlsplit


def normalize_origin(url: str) -> str:
    parsed = urlsplit(url)
    if parsed.scheme not in {"http", "https"} or not parsed.netloc:
        raise ValueError(f"Invalid auth origin: {url}")
    return f"{parsed.scheme}://{parsed.netloc}"


def sha1_hex(value: str) -> str:
    return hashlib.sha1(value.encode("utf-8")).hexdigest()


@dataclass
class AuthContext:
    origin: str
    cookie_header: str
    clock: Callable[[], float] = time.time

    def __post_init__(self) -> None:
        self.origin = normalize_origin(self.origin)
        if not self.cookie_header:
            raise ValueError("AuthContext requires a non-empty cookie header")

    def set_cookie_header(self, cookie_header: str) -> None:
        if not cookie_header:
            raise ValueError("AuthContext requires a non-empty cookie header")
        self.cookie_header = cookie_header

    def cookie(self, name: str) -> str | None:
        prefix = f"{name}="
        for raw_pair in self.cookie_header.split(";"):
            pair = raw_pair.strip()
            if pair.startswith(prefix):
                return pair[len(prefix):]
        return None

    @property
    def unix_timestamp(self) -> int:
        return int(self.clock())


def _proof(context: AuthContext, cookie_value: str, scheme: str, parameters: Sequence[dict[str, str]] | None = None) -> str:
    timestamp = context.unix_timestamp
    if parameters is None:
        digest = sha1_hex(f"{cookie_value} {context.origin}")
        return f"{scheme} {digest}"
    values = [item["value"] for item in parameters]
    prefix = [str(timestamp), cookie_value, context.origin] if not values else [":".join(values), str(timestamp), cookie_value, context.origin]
    suffix = "".join(item["key"] for item in parameters)
    proof = f"{timestamp}_{sha1_hex(' '.join(prefix))}"
    return f"{scheme} {proof}{'_' + suffix if suffix else ''}"


def generate_authorization_header(context: AuthContext, parameters: Sequence[dict[str, str]] | None = ()) -> str | None:
    secure = context.origin.startswith("https:")
    primary = context.cookie("SAPISID") or context.cookie("__Secure-3PAPISID") if secure else context.cookie("APISID") or context.cookie("__Secure-3PAPISID")
    if not primary:
        return None
    parts = [_proof(context, primary, "SAPISIDHASH" if secure else "APISIDHASH", parameters)]
    if secure:
        for name, scheme in (("__Secure-1PAPISID", "SAPISID1PHASH"), ("__Secure-3PAPISID", "SAPISID3PHASH")):
            if value := context.cookie(name):
                parts.append(_proof(context, value, scheme, parameters))
    return " ".join(parts)
