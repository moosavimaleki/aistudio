"""فایل‌های cookie را با ترتیب پایدار از یک directory پیدا می‌کند."""

import re
from pathlib import Path


def discover_cookie_files(directory: str | Path) -> tuple[Path, ...]:
    root = Path(directory).expanduser().resolve()
    if not root.is_dir():
        raise ValueError(f"Cookie directory does not exist: {root}")
    files = tuple(sorted(
        (path for path in root.iterdir() if path.is_file() and path.suffix.lower() == ".txt"),
        key=lambda path: _natural_key(path.name),
    ))
    if not files:
        raise ValueError(f"Cookie directory has no .txt files: {root}")
    return files


def _natural_key(name: str) -> list[object]:
    return [int(part) if part.isdigit() else part.lower() for part in re.split(r"(\d+)", name)]
