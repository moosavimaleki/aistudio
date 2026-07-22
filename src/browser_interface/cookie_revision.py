"""Bind a persistent Chrome profile to its latest cookie source file."""

import hashlib
import os
import tempfile
from pathlib import Path


MARKER_NAME = ".cookie-source.sha256"


def revision_matches(cookie_file: Path, profile_directory: Path) -> bool:
    marker = profile_directory / MARKER_NAME
    if not cookie_file.is_file() or not marker.is_file():
        return False
    return marker.read_text(encoding="ascii").strip() == _digest(cookie_file)


def save_revision(cookie_file: Path, profile_directory: Path) -> None:
    profile_directory.mkdir(parents=True, exist_ok=True)
    marker = profile_directory / MARKER_NAME
    descriptor, temporary_name = tempfile.mkstemp(
        dir=profile_directory,
        prefix=f"{MARKER_NAME}.",
    )
    try:
        with os.fdopen(descriptor, "w", encoding="ascii") as output:
            output.write(_digest(cookie_file))
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary_name, marker)
    except Exception:
        try:
            os.unlink(temporary_name)
        except FileNotFoundError:
            pass
        raise


def _digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()
