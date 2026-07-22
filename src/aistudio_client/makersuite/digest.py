"""Digest متن و file id را مطابق _.bv/_.Di در bundle می‌سازد."""

import hashlib


def content_binding_digest(payload: list) -> str:
    try:
        values = [_part_digest_value(part) for turn in payload[1] for part in turn[0]]
    except (IndexError, TypeError):
        raise ValueError("GenerateContent payload must contain contents at index 1") from None
    return hashlib.sha256(" ".join(values).encode()).hexdigest()


def _part_digest_value(part: object) -> str:
    if not isinstance(part, list):
        return ""
    if len(part) > 1 and isinstance(part[1], str):
        return part[1]
    if len(part) > 5 and isinstance(part[5], list) and part[5]:
        file_id = part[5][0]
        return file_id if isinstance(file_id, str) else ""
    return ""
