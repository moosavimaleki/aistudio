"""Toolهای Vertex را به ساختار positional MakerSuite تبدیل می‌کند."""

from typing import Any

from .schema import encode_schema


def encode_tools(tools: list[Any] | None) -> list | None:
    if tools is None:
        return None
    return [_encode_tool(tool) for tool in tools]


def _encode_tool(tool: Any) -> list:
    if not isinstance(tool, dict):
        raise ValueError("Tool must be an object")
    supported = {"codeExecution", "functionDeclarations"}
    unknown = set(tool) - supported
    if unknown:
        names = ", ".join(sorted(unknown))
        raise ValueError(f"Unsupported Tool fields: {names}")

    encoded = [None, None]
    if tool.get("codeExecution") is not None:
        encoded[0] = []
    declarations = tool.get("functionDeclarations")
    if declarations is not None:
        encoded[1] = [_function_declaration(item) for item in declarations]
    if all(value is None for value in encoded):
        raise ValueError("Tool must set codeExecution or functionDeclarations")
    return _trim(encoded)


def _function_declaration(value: Any) -> list:
    if not isinstance(value, dict) or not value.get("name"):
        raise ValueError("functionDeclarations[].name is required")
    parameters = value.get("parameters")
    if parameters is None:
        parameters = value.get("parametersJsonSchema")
    response = value.get("response")
    if response is None:
        response = value.get("responseJsonSchema")
    return _trim([
        value["name"],
        value.get("description"),
        encode_schema(parameters) if parameters is not None else None,
        encode_schema(response) if response is not None else None,
    ])


def _trim(values: list) -> list:
    while values and values[-1] is None:
        values.pop()
    return values
