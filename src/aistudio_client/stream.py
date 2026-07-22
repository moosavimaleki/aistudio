"""Decoder for the newline-delimited positional GenerateContent stream."""

from __future__ import annotations

import json
from collections.abc import Callable
from typing import Any

from .errors import ClientError
from .models import GenerateResult


def visible_text_from_chunk(chunk: Any) -> str:
    if isinstance(chunk, str):
        return chunk
    if isinstance(chunk, dict):
        return str(chunk.get("text") or chunk.get("delta") or "")
    if not isinstance(chunk, list) or not chunk or not isinstance(chunk[0], list):
        return ""
    text = ""
    for frame in chunk[0]:
        try:
            content = frame[0][0][0]
            if not isinstance(content, list) or content[1] != "model":
                continue
            for part in content[0]:
                if isinstance(part, list) and len(part) > 1 and isinstance(part[1], str):
                    text += part[1]
        except (IndexError, TypeError):
            continue
    return text


def collect_generate_result(response, on_chunk: Callable[[Any], None] | None = None) -> GenerateResult:
    result = GenerateResult()
    for raw_line in response.iter_lines(decode_unicode=True):
        if not raw_line:
            continue
        try:
            chunk = json.loads(raw_line)
        except json.JSONDecodeError:
            chunk = raw_line
        result.chunks.append(chunk)
        result.final_text += visible_text_from_chunk(chunk)
        if isinstance(chunk, dict):
            result.finish_reason = chunk.get("finishReason", result.finish_reason)
            result.usage = chunk.get("usage", result.usage)
            result.conversation_metadata = chunk.get("conversationMetadata", result.conversation_metadata)
        if on_chunk:
            on_chunk(chunk)
    if not result.chunks and response.status_code == 200:
        raise ClientError("GenerateContent returned an empty stream", phase="STREAM")
    return result
