"""Read bootstrap values owned by the loaded AI Studio document."""

import base64
import re
from typing import Any

from ..errors import InvalidCookieSession

RUNTIME_SCRIPT = """() => {
  const data = window.WIZ_global_data || {};
  return {
    apiKey: typeof data.WIu0Nc === "string" ? data.WIu0Nc : null,
    rawVisitId: typeof data.teM9xe === "string" ? data.teM9xe : null,
    attestationEnabled: data.UsvuEb !== false,
  };
}"""


def visit_id(raw_visit_id: str) -> str:
    encoded = base64.urlsafe_b64encode(raw_visit_id.encode()).decode().rstrip("=")
    return f"v1_{encoded}"


async def read_runtime_config(page: Any) -> dict[str, Any]:
    config = await page.evaluate(RUNTIME_SCRIPT)
    if not config.get("apiKey") or not config.get("rawVisitId"):
        config = _read_html_config(await page.content())
    if not config.get("apiKey") or not config.get("rawVisitId"):
        raise InvalidCookieSession(
            "Bootstrap response does not contain WIu0Nc and teM9xe runtime config"
        )
    config["visitId"] = visit_id(config["rawVisitId"])
    return config


def _read_html_config(html: str) -> dict[str, Any]:
    api_key = re.search(r'"WIu0Nc":"([^"\\]+)"', html)
    raw_visit_id = re.search(r'"teM9xe":"([^"\\]+)"', html)
    disabled = re.search(r'"UsvuEb"\s*:\s*(?:false|"false")', html)
    return {
        "apiKey": api_key.group(1) if api_key else None,
        "rawVisitId": raw_visit_id.group(1) if raw_visit_id else None,
        "attestationEnabled": disabled is None,
    }
