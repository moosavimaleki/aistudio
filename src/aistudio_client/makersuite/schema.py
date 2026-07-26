"""بخش تأییدشدهٔ JSON Schema را به Schema داخلی MakerSuite تبدیل می‌کند."""

from typing import Any


TYPE_ENUM = {
    "string": 1,
    "number": 2,
    "integer": 3,
    "boolean": 4,
    "array": 5,
    "object": 6,
    "null": 7,
}

SUPPORTED_KEYS = {
    "type", "format", "description", "nullable", "enum",
    "items", "properties", "required",
}


def encode_schema(schema: dict[str, Any]) -> list:
    if not isinstance(schema, dict):
        raise ValueError("responseSchema must be an object")
    unsupported = set(schema) - SUPPORTED_KEYS
    if unsupported:
        names = ", ".join(sorted(unsupported))
        raise ValueError(f"Unsupported responseSchema fields: {names}")

    schema_type = str(schema.get("type", "")).lower()
    if schema_type not in TYPE_ENUM:
        raise ValueError("responseSchema.type is required and must be supported")

    encoded = [None] * 8
    encoded[0] = TYPE_ENUM[schema_type]
    encoded[1] = schema.get("format")
    encoded[2] = schema.get("description")
    encoded[3] = schema.get("nullable")
    encoded[4] = schema.get("enum")
    if schema.get("items") is not None:
        encoded[5] = encode_schema(schema["items"])
    if schema.get("properties") is not None:
        properties = schema["properties"]
        if not isinstance(properties, dict):
            raise ValueError("responseSchema.properties must be an object")
        encoded[6] = [[name, encode_schema(value)] for name, value in properties.items()]
    encoded[7] = schema.get("required")
    return _trim(encoded)


def _trim(values: list) -> list:
    while values and values[-1] is None:
        values.pop()
    return values
