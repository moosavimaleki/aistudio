"""Environment and cookie-file configuration. No browser globals are used."""

from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path
from urllib.parse import urlsplit

from .cookie_files import discover_cookie_files
from shared import upstream_value


DEFAULT_ENV_FILE = Path(__file__).resolve().parents[2] / ".env"


def parse_env(text: str) -> dict[str, str]:
    values: dict[str, str] = {}
    for raw_line in text.splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        name, value = line.split("=", 1)
        name, value = name.strip(), value.strip()
        if not name:
            continue
        if len(value) >= 2 and value[:1] == value[-1:] and value[:1] in {"'", '"'}:
            value = value[1:-1]
        values[name] = value
    return values


def parse_netscape_cookie_header(text: str, hostname: str | None = None) -> str:
    hostname = hostname or urlsplit(upstream_value("aistudio", "origin")).hostname
    if not hostname:
        raise ValueError("aistudio.origin must contain a hostname")
    pairs: list[str] = []
    for raw_line in text.splitlines():
        line = raw_line.removeprefix("#HttpOnly_")
        if not line or line.startswith("#"):
            continue
        fields = line.split("\t")
        if len(fields) < 7:
            continue
        domain, _, _, _, _, name, *value_parts = fields
        cookie_domain = domain.lstrip(".").lower()
        if hostname != cookie_domain and not hostname.endswith(f".{cookie_domain}"):
            continue
        value = "\t".join(value_parts)
        if name and value:
            pairs.append(f"{name}={value}")
    if not pairs:
        raise ValueError(f"Cookie file contains no cookies for {hostname}")
    return "; ".join(pairs)


@dataclass(frozen=True)
class Settings:
    env_file: Path
    values: dict[str, str]
    cookie_header: str
    origin_url: str
    model: str | None
    token_factory_url: str | None
    waa_api_key: str | None
    proxy_url: str | None
    auth_user: str
    browser_id: str | None

    @classmethod
    def load(cls, env_file: str | Path | None = None) -> "Settings":
        resolved_env_file = Path(env_file or os.environ.get("AISTUDIO_ENV_FILE") or DEFAULT_ENV_FILE).expanduser().resolve()
        text = resolved_env_file.read_text(encoding="utf-8") if resolved_env_file.is_file() else ""
        values = parse_env(text)
        environment_values = {
            name: value
            for name, value in os.environ.items()
            if name.startswith("AISTUDIO_") or name == "TOKEN_FACTORY_URL"
        }
        values.update(environment_values)

        cookie_dir = Path(values.get("AISTUDIO_COOKIE_DIR", resolved_env_file.parent / "COOKIES"))
        if not cookie_dir.is_absolute():
            cookie_dir = resolved_env_file.parent / cookie_dir
        cookie_dir = cookie_dir.resolve()
        values["AISTUDIO_COOKIE_DIR"] = str(cookie_dir)
        first_cookie = discover_cookie_files(cookie_dir)[0]
        cookie_header = parse_netscape_cookie_header(first_cookie.read_text(encoding="utf-8"))

        return cls(
            env_file=resolved_env_file,
            values=values,
            cookie_header=cookie_header,
            origin_url=upstream_value("aistudio", "bootstrap_url"),
            model=values.get("AISTUDIO_MODEL"),
            token_factory_url=values.get("TOKEN_FACTORY_URL"),
            waa_api_key=upstream_value("opaque", "waa_api_key"),
            proxy_url=values.get("AISTUDIO_PROXY_URL", "http://127.0.0.1:10808"),
            auth_user=values.get("AISTUDIO_AUTH_USER", "0"),
            browser_id=values.get("AISTUDIO_BROWSER_ID"),
        )
