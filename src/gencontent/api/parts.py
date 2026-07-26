"""مدل‌های Vertex مربوط به Content Part."""

from typing import Any, Literal

from pydantic import Field, model_validator

from .base import ApiModel


class FileData(ApiModel):
    file_id: str = Field(min_length=1, alias="fileId")
    mime_type: str | None = Field(default=None, alias="mimeType")


class InlineData(ApiModel):
    mime_type: str = Field(min_length=1, alias="mimeType")
    data: str = Field(min_length=1)
    display_name: str | None = Field(default=None, alias="displayName")


class FunctionCall(ApiModel):
    name: str = Field(min_length=1)
    args: dict[str, Any] | None = None
    id: str | None = None


class FunctionResponse(ApiModel):
    name: str = Field(min_length=1)
    response: dict[str, Any]
    id: str | None = None


class ExecutableCode(ApiModel):
    language: Literal["LANGUAGE_UNSPECIFIED", "PYTHON"]
    code: str


class CodeExecutionResult(ApiModel):
    outcome: Literal[
        "OUTCOME_UNSPECIFIED",
        "OUTCOME_OK",
        "OUTCOME_FAILED",
        "OUTCOME_DEADLINE_EXCEEDED",
    ]
    output: str


class VideoMetadata(ApiModel):
    start_offset: str | None = Field(default=None, alias="startOffset")
    end_offset: str | None = Field(default=None, alias="endOffset")
    fps: float | None = Field(default=None, gt=0)


class Part(ApiModel):
    text: str | None = None
    file_data: FileData | None = Field(default=None, alias="fileData")
    inline_data: InlineData | None = Field(default=None, alias="inlineData")
    function_call: FunctionCall | None = Field(default=None, alias="functionCall")
    function_response: FunctionResponse | None = Field(
        default=None,
        alias="functionResponse",
    )
    executable_code: ExecutableCode | None = Field(
        default=None,
        alias="executableCode",
    )
    code_execution_result: CodeExecutionResult | None = Field(
        default=None,
        alias="codeExecutionResult",
    )
    video_metadata: VideoMetadata | None = Field(default=None, alias="videoMetadata")
    thought: bool | None = None
    thought_signature: str | None = Field(default=None, alias="thoughtSignature")
    raw: list[Any] | None = None

    @model_validator(mode="after")
    def exactly_one_payload(self):
        payloads = (
            self.text,
            self.file_data,
            self.inline_data,
            self.function_call,
            self.function_response,
            self.executable_code,
            self.code_execution_result,
            self.raw,
        )
        if sum(value is not None for value in payloads) != 1:
            raise ValueError("Part must set exactly one payload")
        if self.video_metadata and not (self.file_data or self.inline_data):
            raise ValueError("videoMetadata requires fileData or inlineData")
        return self
