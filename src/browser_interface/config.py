"""Load independent browser accounts from the environment."""

import os
import re
from dataclasses import dataclass
from pathlib import Path

from aistudio_client.cookie_files import discover_cookie_files
from .cookie_source import read_cookie_file

BROWSER_ID_PATTERN = re.compile(r"^[a-zA-Z0-9_-]+$")


@dataclass(frozen=True)
class BrowserSpec:
    browser_id: str
    auth_user: str = "0"
    cookie_header: str | None = None
    cookie_file: Path | None = None


@dataclass(frozen=True)
class BrowserConfig:
    browsers: tuple[BrowserSpec, ...]
    default_browser_id: str
    cdp_base_port: int


def load_browser_config() -> BrowserConfig:
    values = _environment_values()
    specs = _cookie_directory_specs(values)
    _validate_specs(specs)
    default_id = os.getenv("AISTUDIO_DEFAULT_BROWSER_ID", "").strip()
    default_id = default_id or specs[0].browser_id
    if default_id not in {spec.browser_id for spec in specs}:
        raise ValueError(f"Unknown default browserId: {default_id}")
    return BrowserConfig(
        browsers=specs,
        default_browser_id=default_id,
        cdp_base_port=int(os.getenv("CHROME_CDP_BASE_PORT", "9223")),
    )


def _cookie_directory_specs(values: dict[str, str]) -> tuple[BrowserSpec, ...]:
    directory = values.get("AISTUDIO_COOKIE_DIR", "/app/cookies")
    specs = []
    for index, path in enumerate(discover_cookie_files(directory), start=1):
        suffix = "" if index == 1 else str(index)
        browser_id = values.get(f"AISTUDIO_BROWSER_ID{suffix}")
        browser_id = browser_id or ("default" if index == 1 else f"browser{index}")
        auth_user = values.get(f"AISTUDIO_AUTH_USER{suffix}", "0") or "0"
        specs.append(
            BrowserSpec(
                browser_id=browser_id,
                auth_user=auth_user,
                cookie_header=read_cookie_file(str(path)),
                cookie_file=path,
            )
        )
    return tuple(specs)


def _environment_values() -> dict[str, str]:
    values: dict[str, str] = {}
    env_file = os.getenv("AISTUDIO_ENV_FILE", "").strip()
    if env_file and Path(env_file).is_file():
        for source_line in Path(env_file).read_text(encoding="utf-8").splitlines():
            line = source_line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            name, value = (part.strip() for part in line.split("=", 1))
            values[name] = value.strip("\"'")
    values.update(os.environ)
    return values


def _validate_specs(specs: tuple[BrowserSpec, ...]) -> None:
    ids = [spec.browser_id for spec in specs]
    if any(not BROWSER_ID_PATTERN.fullmatch(browser_id) for browser_id in ids):
        raise ValueError("Browser ids may contain only letters, digits, underscore and dash")
    if len(ids) != len(set(ids)):
        raise ValueError("Browser configuration contains duplicate browserId values")
