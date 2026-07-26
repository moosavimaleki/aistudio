"""Partهای نام‌دار Vertex را به fieldهای قطعی MakerSuite تبدیل می‌کند."""

import re
from copy import deepcopy
from typing import Any

from .value import encode_struct


LANGUAGE_ENUM = {"LANGUAGE_UNSPECIFIED": 0, "PYTHON": 1}
OUTCOME_ENUM = {
    "OUTCOME_UNSPECIFIED": 0,
    "OUTCOME_OK": 1,
    "OUTCOME_FAILED": 2,
    "OUTCOME_DEADLINE_EXCEEDED": 3,
}
_DURATION = re.compile(r"^(\d+)(?:\.(\d{1,9}))?s$")


def encode_named_part(part: dict[str, Any]) -> list:
    if part.get("raw") is not None:
        return deepcopy(part["raw"])

    encoded = [None] * 16
    if part.get("text") is not None:
        encoded[1] = part["text"]
    elif part.get("fileData") is not None:
        encoded[5] = [part["fileData"]["fileId"]]
    elif part.get("functionCall") is not None:
        encoded[10] = _function_call(part["functionCall"])
    elif part.get("functionResponse") is not None:
        encoded[11] = _function_response(part["functionResponse"])
    elif part.get("executableCode") is not None:
        encoded[7] = _executable_code(part["executableCode"])
    elif part.get("codeExecutionResult") is not None:
        encoded[8] = _execution_result(part["codeExecutionResult"])
    else:
        raise ValueError("Unsupported Part payload")

    encoded[12] = part.get("thought")
    encoded[14] = part.get("thoughtSignature")
    if part.get("videoMetadata") is not None:
        encoded[15] = _video_metadata(part["videoMetadata"])
    return _trim(encoded)


def _function_call(value: dict[str, Any]) -> list:
    return _trim([
        value["name"],
        encode_struct(value.get("args") or {}),
        value.get("id"),
    ])


def _function_response(value: dict[str, Any]) -> list:
    return _trim([
        value["name"],
        encode_struct(value["response"]),
        value.get("id"),
    ])


def _executable_code(value: dict[str, Any]) -> list:
    return [LANGUAGE_ENUM[value["language"]], value["code"]]


def _execution_result(value: dict[str, Any]) -> list:
    return [OUTCOME_ENUM[value["outcome"]], value["output"]]


def _video_metadata(value: dict[str, Any]) -> list:
    return _trim([
        _duration(value.get("startOffset")),
        _duration(value.get("endOffset")),
        value.get("fps"),
    ])


def _duration(value: str | None) -> list | None:
    if value is None:
        return None
    match = _DURATION.fullmatch(value)
    if not match:
        raise ValueError("Video offsets must use protobuf duration syntax such as 1.5s")
    seconds = int(match.group(1))
    nanos = int((match.group(2) or "").ljust(9, "0") or 0)
    return _trim([seconds, nanos or None])


def _trim(values: list) -> list:
    while values and values[-1] is None:
        values.pop()
    return values
