"""Optional same-browser diagnostic probe."""

import copy
from typing import Any

from shared import upstream_value

PROBE_SCRIPT = """async ({url, requestHeaders, requestPayload}) => {
  try {
    const response = await fetch(url, {
      method: "POST",
      credentials: "include",
      headers: requestHeaders,
      body: JSON.stringify(requestPayload),
    });
    return {status: response.status, body: await response.text()};
  } catch (error) {
    return {networkError: error instanceof Error ? error.message : String(error)};
  }
}"""

ALLOWED_HEADERS = {
    "accept", "accept-language", "authorization", "content-type",
    "x-browser-channel", "x-browser-copyright", "x-browser-validation",
    "x-browser-year", "x-client-data", "x-aistudio-visit-id",
    "x-goog-api-key", "x-goog-authuser",
    "x-goog-visitor-id", "x-user-agent",
}
ALLOWED_HEADERS.add(upstream_value("makersuite", "logging_context_header").lower())


async def probe_generate(
    page: Any,
    latest_rpc_headers: dict[str, Any],
    runtime: dict[str, Any],
    generate_request: dict[str, Any],
    token: str,
) -> dict[str, Any]:
    payload = generate_request.get("payload")
    url = generate_request.get("url")
    if not isinstance(payload, list) or not isinstance(url, str):
        raise ValueError("GenerateContent request context is invalid for probe")

    headers = _allowed(generate_request.get("headers", {}))
    headers.update(_allowed(latest_rpc_headers))
    headers["x-goog-api-key"] = runtime["apiKey"]
    headers["x-aistudio-visit-id"] = runtime["visitId"]
    headers["x-goog-authuser"] = str(runtime.get("authUser", "0"))

    request_payload = copy.deepcopy(payload)
    request_payload[4] = token
    return await page.evaluate(
        PROBE_SCRIPT,
        {"url": url, "requestHeaders": headers, "requestPayload": request_payload},
    )


def _allowed(headers: dict[str, Any]) -> dict[str, str]:
    return {
        str(name).lower(): str(value)
        for name, value in headers.items()
        if str(name).lower() in ALLOWED_HEADERS
    }
