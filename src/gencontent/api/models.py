"""قراردادهای HTTP برای API ساده و API شبیه Vertex."""

from typing import Any, Literal

from pydantic import BaseModel, ConfigDict, Field, model_validator


class ApiModel(BaseModel):
    model_config = ConfigDict(extra="forbid", populate_by_name=True)


class FileData(ApiModel):
    file_id: str = Field(min_length=1, alias="fileId")
    mime_type: str | None = Field(default=None, alias="mimeType")


class InlineData(ApiModel):
    mime_type: str = Field(min_length=1, alias="mimeType")
    data: str = Field(min_length=1)
    display_name: str | None = Field(default=None, alias="displayName")


class Part(ApiModel):
    text: str | None = None
    file_data: FileData | None = Field(default=None, alias="fileData")
    inline_data: InlineData | None = Field(default=None, alias="inlineData")
    raw: list[Any] | None = None

    @model_validator(mode="after")
    def exactly_one_value(self):
        count = sum(value is not None for value in (
            self.text, self.file_data, self.inline_data, self.raw,
        ))
        if count != 1:
            raise ValueError("Part must set exactly one of text, fileData, inlineData, or raw")
        return self


class Content(ApiModel):
    role: Literal["user", "model"]
    parts: list[Part] = Field(min_length=1)


class SystemInstruction(ApiModel):
    parts: list[Part] = Field(min_length=1)


class ThinkingConfig(ApiModel):
    include_thoughts: bool = Field(default=False, alias="includeThoughts")
    thinking_budget: int | None = Field(default=None, alias="thinkingBudget")
    level_enum: int | None = Field(default=None, alias="levelEnum")

    @model_validator(mode="after")
    def one_thinking_mode(self):
        if self.thinking_budget is not None and self.level_enum is not None:
            raise ValueError("thinkingBudget and levelEnum cannot be used together")
        if self.thinking_budget is None and self.level_enum is None:
            raise ValueError("thinkingBudget or levelEnum is required")
        return self


class GenerationConfig(ApiModel):
    stop_sequences: list[str] | None = Field(default=None, alias="stopSequences")
    max_output_tokens: int | None = Field(default=None, gt=0, alias="maxOutputTokens")
    temperature: float | None = Field(default=None, ge=0)
    top_p: float | None = Field(default=None, ge=0, le=1, alias="topP")
    top_k: int | None = Field(default=None, gt=0, alias="topK")
    response_mime_type: str | None = Field(default=None, alias="responseMimeType")
    response_schema: dict[str, Any] | None = Field(default=None, alias="responseSchema")
    response_modalities: list[int] | None = Field(default=None, alias="responseModalities")
    speech_config: list[Any] | None = Field(default=None, alias="speechConfig")
    thinking_config: ThinkingConfig = Field(alias="thinkingConfig")
    media_resolution: int | None = Field(default=None, alias="mediaResolution")
    seed: int | None = None
    candidate_count: int | None = Field(default=None, alias="candidateCount")

    @model_validator(mode="after")
    def reject_unmapped_candidate_count(self):
        if self.candidate_count is not None:
            raise ValueError("candidateCount is not mapped in the staging wire format")
        return self


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
