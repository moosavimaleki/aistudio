"""Persist the live Chrome cookie jar as a Netscape cookie file."""

import os
import tempfile
from pathlib import Path
from typing import Any


def persist_cookie_file(path: Path, cookies: list[dict[str, Any]]) -> int:
    """Atomically replace one profile's source file with Chrome's live state."""

    records = []
    for cookie in cookies:
        record = _netscape_record(cookie)
        if record:
            records.append(record)
    records.sort(key=lambda record: (record[0], record[2], record[5]))
    content = "# Netscape HTTP Cookie File\n" + "".join(
        "\t".join(record) + "\n" for record in records
    )

    path.parent.mkdir(parents=True, exist_ok=True)
    mode = path.stat().st_mode & 0o777 if path.exists() else 0o600
    directory_group = path.parent.stat().st_gid
    descriptor, temporary_name = tempfile.mkstemp(
        dir=path.parent,
        prefix=f".{path.name}.",
        suffix=".tmp",
    )
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8") as output:
            output.write(content)
            output.flush()
            os.fsync(output.fileno())
        os.chmod(temporary_name, mode)
        os.chown(temporary_name, -1, directory_group)
        os.replace(temporary_name, path)
    except Exception:
        try:
            os.unlink(temporary_name)
        except FileNotFoundError:
            pass
        raise
    return len(records)


def _netscape_record(cookie: dict[str, Any]) -> tuple[str, ...]:
    domain = _clean(cookie.get("domain", ""))
    name = _clean(cookie.get("name", ""))
    value = _clean(cookie.get("value", ""))
    normalized = domain.removeprefix(".").lower()
    if (
        not domain
        or not name
        or name.startswith("__Host-")
        or (normalized != "google.com" and not normalized.endswith(".google.com"))
    ):
        return ()

    if cookie.get("httpOnly"):
        domain = f"#HttpOnly_{domain}"
    expires = cookie.get("expires", 0)
    expires = str(max(0, int(expires))) if isinstance(expires, (int, float)) else "0"
    return (
        domain,
        "TRUE" if domain.removeprefix("#HttpOnly_").startswith(".") else "FALSE",
        _clean(cookie.get("path", "/")) or "/",
        "TRUE" if cookie.get("secure") else "FALSE",
        expires,
        name,
        value,
    )


def _clean(value: Any) -> str:
    return str(value or "").replace("\t", "").replace("\r", "").replace("\n", "")
