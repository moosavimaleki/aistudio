"""Request سطح اول GenerateContent را بدون attestation نهایی assemble می‌کند."""

from typing import Any

from ..models import GenerateInput
from .content import encode_contents, encode_system_instruction
from .generation import encode_generation_config


DEFAULT_SAFETY_SETTINGS = [
    [None, None, 7, 5],
    [None, None, 8, 5],
    [None, None, 9, 5],
    [None, None, 10, 5],
]


def build_generate_content_payload(input: GenerateInput) -> list:
    if not input.model.startswith("models/"):
        raise ValueError("model must start with models/")

    contents = input.contents or _legacy_contents(input)
    payload = [None] * 11
    payload[0] = input.model
    payload[1] = encode_contents(contents)
    payload[2] = (
        DEFAULT_SAFETY_SETTINGS if input.safety_settings is None else input.safety_settings
    )
    payload[3] = encode_generation_config(input.generation_config)
    payload[5] = encode_system_instruction(input.system_instruction)
    payload[6] = input.tools
    payload[10] = 1

    if input.continuation_token is not None:
        _set_field(payload, 11, input.continuation_token)
    if input.tool_context is not None:
        _set_field(payload, 13, input.tool_context)
    return payload


def _legacy_contents(input: GenerateInput) -> list[dict[str, Any]]:
    latest = input.latest_user_turn or {"role": "user", "text": input.prompt}
    if latest.get("role") != "user":
        raise ValueError("latest_user_turn role must be 'user'")
    return [*input.history, latest]


def _set_field(payload: list, index: int, value: Any) -> None:
    if len(payload) <= index:
        payload.extend([None] * (index + 1 - len(payload)))
    payload[index] = value
