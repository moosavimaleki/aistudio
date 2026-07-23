"""Part positional پاسخ MakerSuite را به شکل Vertex برمی‌گرداند."""

from typing import Any


LANGUAGE_NAME = {0: "LANGUAGE_UNSPECIFIED", 1: "PYTHON"}
OUTCOME_NAME = {
    0: "OUTCOME_UNSPECIFIED",
    1: "OUTCOME_OK",
    2: "OUTCOME_FAILED",
    3: "OUTCOME_DEADLINE_EXCEEDED",
}


def decode_part(part: list) -> dict[str, Any] | None:
    decoded = _payload(part)
    if decoded is None:
        return None
    if len(part) > 12 and isinstance(part[12], bool):
        decoded["thought"] = part[12]
    if len(part) > 14 and isinstance(part[14], str):
        decoded["thoughtSignature"] = part[14]
    if len(part) > 15 and isinstance(part[15], list):
        decoded["videoMetadata"] = _video_metadata(part[15])
    return decoded


def _payload(part: list) -> dict[str, Any] | None:
    if len(part) > 1 and isinstance(part[1], str):
        return {"text": part[1]}
    if len(part) > 5 and isinstance(part[5], list) and part[5]:
        return {"fileData": {"fileId": part[5][0]}}
    if len(part) > 7 and isinstance(part[7], list):
        value = part[7]
        return {"executableCode": {
            "language": LANGUAGE_NAME.get(_item(value, 0), "LANGUAGE_UNSPECIFIED"),
            "code": _item(value, 1, ""),
        }}
    if len(part) > 8 and isinstance(part[8], list):
        value = part[8]
        return {"codeExecutionResult": {
            "outcome": OUTCOME_NAME.get(_item(value, 0), "OUTCOME_UNSPECIFIED"),
            "output": _item(value, 1, ""),
        }}
    if len(part) > 10 and isinstance(part[10], list):
        return {"functionCall": _call(part[10])}
    if len(part) > 11 and isinstance(part[11], list):
        return {"functionResponse": _response(part[11])}
    return None


def _call(value: list) -> dict[str, Any]:
    result = {"name": _item(value, 0, ""), "args": decode_struct(_item(value, 1, []))}
    if _item(value, 2) is not None:
        result["id"] = value[2]
    return result


def _response(value: list) -> dict[str, Any]:
    result = {
        "name": _item(value, 0, ""),
        "response": decode_struct(_item(value, 1, [])),
    }
    if _item(value, 2) is not None:
        result["id"] = value[2]
    return result


def decode_struct(value: Any) -> dict[str, Any]:
    entries = _item(value, 0, [])
    if not isinstance(entries, list):
        return {}
    return {
        str(entry[0]): _decode_value(entry[1])
        for entry in entries
        if isinstance(entry, list) and len(entry) > 1
    }


def _decode_value(value: Any) -> Any:
    if not isinstance(value, list):
        return None
    if _item(value, 0) is not None:
        return None
    if _item(value, 1) is not None:
        return value[1]
    if _item(value, 2) is not None:
        return value[2]
    if _item(value, 3) is not None:
        return value[3]
    if isinstance(_item(value, 4), list):
        return decode_struct(value[4])
    nested = _item(value, 5)
    items = _item(nested, 0, [])
    return [_decode_value(item) for item in items] if isinstance(items, list) else []


def _video_metadata(value: list) -> dict[str, Any]:
    result: dict[str, Any] = {}
    if isinstance(_item(value, 0), list):
        result["startOffset"] = _duration(value[0])
    if isinstance(_item(value, 1), list):
        result["endOffset"] = _duration(value[1])
    if _item(value, 2) is not None:
        result["fps"] = value[2]
    return result


def _duration(value: list) -> str:
    seconds = int(_item(value, 0, 0))
    nanos = int(_item(value, 1, 0))
    suffix = f".{nanos:09d}".rstrip("0") if nanos else ""
    return f"{seconds}{suffix}s"


def _item(value: Any, index: int, default=None):
    return value[index] if isinstance(value, list) and len(value) > index else default
