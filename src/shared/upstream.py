"""منبع واحد مقادیر متغیر upstream را از YAML می‌خواند."""

from functools import lru_cache
import os
from pathlib import Path

import yaml


@lru_cache(maxsize=1)
def upstream_config() -> dict:
    path = _config_path()
    data = yaml.safe_load(path.read_text(encoding="utf-8"))
    if not isinstance(data, dict) or data.get("version") != 1:
        raise ValueError(f"Invalid upstream config: {path}")
    return data


def upstream_value(section: str, name: str) -> str:
    value = upstream_config().get(section, {}).get(name)
    if not isinstance(value, str) or not value:
        raise ValueError(f"Missing upstream config value: {section}.{name}")
    return value


def _config_path() -> Path:
    explicit = os.getenv("AISTUDIO_UPSTREAM_CONFIG", "").strip()
    candidates = [
        Path(explicit) if explicit else None,
        Path("/app/config/upstream.yaml"),
        Path(__file__).resolve().parents[2] / "config" / "upstream.yaml",
    ]
    for candidate in candidates:
        if candidate and candidate.is_file():
            return candidate
    raise FileNotFoundError("config/upstream.yaml was not found")
