"""مدل‌های Vertex مربوط به GenerationConfig."""

from typing import Any, Literal

from pydantic import Field, model_validator

from .base import ApiModel


class ThinkingConfig(ApiModel):
    include_thoughts: bool = Field(default=False, alias="includeThoughts")
    thinking_budget: int | None = Field(default=None, alias="thinkingBudget")
    level_enum: int | None = Field(default=None, alias="levelEnum")
    thinking_level: Literal["MINIMAL", "LOW", "MEDIUM", "HIGH"] | None = Field(
        default=None,
        alias="thinkingLevel",
    )

    @model_validator(mode="after")
    def one_thinking_mode(self):
        modes = (self.thinking_budget, self.level_enum, self.thinking_level)
        if sum(value is not None for value in modes) != 1:
            raise ValueError(
                "Exactly one of thinkingBudget, levelEnum, or thinkingLevel is required"
            )
        return self


class GenerationConfig(ApiModel):
    stop_sequences: list[str] | None = Field(default=None, alias="stopSequences")
    max_output_tokens: int | None = Field(default=None, gt=0, alias="maxOutputTokens")
    temperature: float | None = Field(default=None, ge=0)
    top_p: float | None = Field(default=None, ge=0, le=1, alias="topP")
    top_k: int | None = Field(default=None, gt=0, alias="topK")
    response_mime_type: str | None = Field(default=None, alias="responseMimeType")
    response_schema: dict[str, Any] | None = Field(default=None, alias="responseSchema")
    response_modalities: list[int] | None = Field(
        default=None,
        alias="responseModalities",
    )
    speech_config: list[Any] | None = Field(default=None, alias="speechConfig")
    thinking_config: ThinkingConfig = Field(alias="thinkingConfig")
    media_resolution: int | None = Field(default=None, alias="mediaResolution")
    seed: int | None = None
    candidate_count: int | None = Field(default=None, gt=0, alias="candidateCount")
