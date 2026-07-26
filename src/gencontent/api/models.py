"""قراردادهای سطح request برای API ساده و Vertex."""

from typing import Any, Literal

from pydantic import Field

from .base import ApiModel
from .generation import GenerationConfig
from .parts import Part


class Content(ApiModel):
    role: Literal["user", "model"]
    parts: list[Part] = Field(min_length=1)


class SystemInstruction(ApiModel):
    parts: list[Part] = Field(min_length=1)


class LabContext(ApiModel):
    continuation_token: Any = Field(default=None, alias="continuationToken")
    tool_context: Any = Field(default=None, alias="toolContext")


class VertexGenerateContentBody(ApiModel):
    contents: list[Content] = Field(min_length=1)
    system_instruction: SystemInstruction | None = Field(default=None, alias="systemInstruction")
    generation_config: GenerationConfig = Field(alias="generationConfig")
    safety_settings: list[Any] | None = Field(default=None, alias="safetySettings")
    tools: list[Any] | None = None
    lab_context: LabContext | None = Field(default=None, alias="labContext")


class GenerateContentBody(ApiModel):
    prompt: str = Field(min_length=1)
    model: str | None = None
    history: list[dict[str, Any]] = Field(default_factory=list)
    generation_config: GenerationConfig = Field(alias="generationConfig")
    safety_settings: list[Any] | None = Field(default=None, alias="safetySettings")
    system_instruction: SystemInstruction | None = Field(default=None, alias="systemInstruction")
