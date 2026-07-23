"""Small explicit data contracts shared between the client modules."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any


@dataclass(frozen=True)
class RuntimeConfig:
    api_key: str
    visit_id: str
    auth_user: str = "0"
    attestation_enabled: bool = True


@dataclass(frozen=True)
class GenerateInput:
    model: str
    prompt: str | None = None
    contents: list[dict[str, Any]] = field(default_factory=list)
    history: list[dict[str, Any]] = field(default_factory=list)
    latest_user_turn: dict[str, Any] | None = None
    generation_config: dict[str, Any] = field(default_factory=dict)
    safety_settings: list[Any] | None = None
    system_instruction: str | dict[str, Any] | None = None
    tools: list[Any] | None = None
    continuation_token: Any = None
    tool_context: Any = None


@dataclass
class GenerateResult:
    final_text: str = ""
    chunks: list[Any] = field(default_factory=list)
    model_parts: list[dict[str, Any]] = field(default_factory=list)
    finish_reason: Any = None
    usage: Any = None
    conversation_metadata: Any = None
