"""مدل HTTP را به GenerateInput و پاسخ داخلی را به پاسخ Vertex تبدیل می‌کند."""

from aistudio_client.models import GenerateInput

from .models import GenerateContentBody, VertexGenerateContentBody


def legacy_input(model: str, body: GenerateContentBody) -> GenerateInput:
    return GenerateInput(
        model=_model_name(model),
        prompt=body.prompt,
        history=body.history,
        generation_config=_dump(body.generation_config),
        safety_settings=body.safety_settings,
        system_instruction=_dump(body.system_instruction),
    )


def vertex_input(model: str, body: VertexGenerateContentBody) -> GenerateInput:
    lab = body.lab_context
    return GenerateInput(
        model=_model_name(model),
        contents=[_dump(content) for content in body.contents],
        generation_config=_dump(body.generation_config),
        safety_settings=body.safety_settings,
        system_instruction=_dump(body.system_instruction),
        tools=body.tools,
        continuation_token=lab.continuation_token if lab else None,
        tool_context=lab.tool_context if lab else None,
    )


def vertex_response(model: str, outcome) -> dict:
    candidate = {
        "content": {
            "role": "model",
            "parts": [{"text": outcome.result.final_text}],
        }
    }
    if outcome.result.finish_reason is not None:
        candidate["finishReason"] = outcome.result.finish_reason

    response = {
        "candidates": [candidate],
        "modelVersion": _model_name(model),
        "labMetadata": {
            "tabId": outcome.tab_id,
            "browserId": outcome.browser_id,
            "authUser": outcome.auth_user,
            "tabGenerateCount": outcome.generate_count,
            "chunkCount": len(outcome.result.chunks),
            "conversationMetadata": outcome.result.conversation_metadata,
        },
    }
    if outcome.result.usage is not None:
        response["usageMetadata"] = outcome.result.usage
    return response


def _model_name(model: str) -> str:
    return model if model.startswith("models/") else f"models/{model}"


def _dump(value):
    if value is None:
        return None
    return value.model_dump(by_alias=True, exclude_none=True)
