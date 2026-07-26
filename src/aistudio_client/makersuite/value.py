"""مقدار JSON را به google.protobuf.Value positional تبدیل می‌کند."""

from typing import Any


def encode_struct(value: dict[str, Any]) -> list:
    return [[[key, encode_value(item)] for key, item in value.items()]]


def encode_value(value: Any) -> list:
    if value is None:
        return [0]
    if isinstance(value, bool):
        return [None, None, None, value]
    if isinstance(value, (int, float)):
        return [None, value]
    if isinstance(value, str):
        return [None, None, value]
    if isinstance(value, dict):
        return [None, None, None, None, encode_struct(value)]
    if isinstance(value, list):
        return [None, None, None, None, None, [[encode_value(item) for item in value]]]
    raise ValueError(f"Unsupported protobuf Struct value: {type(value).__name__}")
