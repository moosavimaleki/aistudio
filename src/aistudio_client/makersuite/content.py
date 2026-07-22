"""Content و Part سطح بالا را به ساختار positional تبدیل می‌کند."""

from copy import deepcopy
from typing import Any


def encode_contents(contents: list[dict[str, Any]]) -> list:
    if not contents:
        raise ValueError("contents must contain at least one turn")
    return [encode_content(content) for content in contents]


def encode_content(content: dict[str, Any]) -> list:
    role = content.get("role")
    if role not in {"user", "model"}:
        raise ValueError("Content role must be 'user' or 'model'")
    parts = content.get("parts")
    if parts is None:
        parts = [content.get("text")]
    if not isinstance(parts, list) or not parts:
        raise ValueError("Content parts must be a non-empty list")
    return [[encode_part(part) for part in parts], role]


def encode_part(part: Any) -> list:
    if isinstance(part, list):
        return deepcopy(part)
    if isinstance(part, str):
        return _text_part(part)
    if not isinstance(part, dict):
        raise ValueError("Part must be text, an object, or a raw positional list")

    choices = [name for name in ("text", "fileData", "raw") if part.get(name) is not None]
    if len(choices) != 1:
        raise ValueError("Part must set exactly one of text, fileData, or raw")
    if choices[0] == "text":
        return _text_part(part["text"])
    if choices[0] == "raw":
        if not isinstance(part["raw"], list):
            raise ValueError("Part raw value must be a positional list")
        return deepcopy(part["raw"])
    return _file_part(part["fileData"])


def encode_system_instruction(value: str | dict[str, Any] | None) -> list | None:
    if value is None:
        return None
    parts = [{"text": value}] if isinstance(value, str) else value.get("parts")
    if not isinstance(parts, list) or not parts:
        raise ValueError("systemInstruction must contain parts")
    return [[encode_part(part) for part in parts], "user"]


def _text_part(value: Any) -> list:
    if not isinstance(value, str) or not value.strip():
        raise ValueError("Text parts must be non-empty strings")
    return [None, value]


def _file_part(value: Any) -> list:
    if not isinstance(value, dict):
        raise ValueError("fileData must be an object")
    file_id = value.get("fileId")
    if not isinstance(file_id, str) or not file_id.strip():
        raise ValueError("fileData.fileId is required before payload encoding")
    return [None, None, None, None, None, [file_id]]
