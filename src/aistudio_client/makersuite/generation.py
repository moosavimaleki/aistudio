"""GenerationConfig عمومی را در شماره‌فیلدهای قطعی MakerSuite قرار می‌دهد."""

from typing import Any

from .schema import encode_schema


DEFAULT_MAX_OUTPUT_TOKENS = 65536
DEFAULT_TEMPERATURE = 1
DEFAULT_TOP_P = 0.95
DEFAULT_TOP_K = 64
THINKING_LEVEL_ENUM = {"LOW": 1, "MEDIUM": 2, "HIGH": 3, "MINIMAL": 4}


def encode_generation_config(config: dict[str, Any]) -> list:
    config = config or {}
    encoded = [None] * 19
    encoded[0] = _value(config, "candidateCount", "candidate_count")
    encoded[1] = _value(config, "stopSequences", "stop_sequences")
    encoded[3] = _value(
        config, "maxOutputTokens", "max_output_tokens", default=DEFAULT_MAX_OUTPUT_TOKENS,
    )
    encoded[4] = _value(config, "temperature", default=DEFAULT_TEMPERATURE)
    encoded[5] = _value(config, "topP", "top_p", default=DEFAULT_TOP_P)
    encoded[6] = _value(config, "topK", "top_k", default=DEFAULT_TOP_K)
    encoded[7] = _value(config, "responseMimeType", "response_mime_type")

    response_schema = _value(config, "responseSchema", "response_schema")
    if response_schema is not None:
        encoded[7] = encoded[7] or "application/json"
        encoded[8] = encode_schema(response_schema)

    encoded[14] = _value(config, "responseModalities", "response_modalities")
    encoded[15] = _value(config, "speechConfig", "speech_config")
    thinking = _value(config, "thinkingConfig", "thinking_config")
    encoded[16] = _thinking_config(thinking)
    encoded[17] = _value(config, "mediaResolution", "media_resolution")
    encoded[18] = _value(config, "seed")
    return _trim(encoded)


def _thinking_config(value: Any) -> list:
    if value is None:
        raise ValueError("thinkingConfig must set thinkingBudget or levelEnum")
    if not isinstance(value, dict):
        raise ValueError("thinkingConfig must be an object")

    budget = _value(value, "thinkingBudget", "thinking_budget")
    level = _value(value, "levelEnum", "level_enum", "thinkingLevel", "thinking_level")
    if budget is not None and level is not None:
        raise ValueError("thinkingConfig cannot set budget and level together")
    if budget is None and level is None:
        raise ValueError("thinkingConfig must set thinkingBudget or levelEnum")
    if isinstance(level, str):
        try:
            level = THINKING_LEVEL_ENUM[level.upper()]
        except KeyError:
            raise ValueError(f"Unsupported thinkingLevel: {level}") from None
    if level is not None and not isinstance(level, int):
        raise ValueError("thinkingLevel must be a known string or numeric levelEnum")
    include = _value(value, "includeThoughts", "include_thoughts", default=False)
    return [bool(include), budget, None, level]


def _value(source: dict[str, Any], *names: str, default: Any = None) -> Any:
    for name in names:
        if name in source:
            return source[name]
    return default


def _trim(values: list) -> list:
    while values and values[-1] is None:
        values.pop()
    return values
