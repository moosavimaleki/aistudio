"""Read bootstrap values owned by the loaded AI Studio document."""

import base64
import json
import re
from typing import Any

from ..errors import InvalidCookieSession
from shared import upstream_value

RUNTIME_GLOBAL = upstream_value("runtime", "global_object")
API_KEY_PROPERTY = upstream_value("runtime", "api_key_property")
VISIT_ID_PROPERTY = upstream_value("runtime", "visit_id_property")
ATTESTATION_PROPERTY = upstream_value("runtime", "attestation_enabled_property")

RUNTIME_SCRIPT = f"""() => {{
  const data = window[{json.dumps(RUNTIME_GLOBAL)}] || {{}};
  return {{
    apiKey: typeof data[{json.dumps(API_KEY_PROPERTY)}] === "string" ? data[{json.dumps(API_KEY_PROPERTY)}] : null,
    rawVisitId: typeof data[{json.dumps(VISIT_ID_PROPERTY)}] === "string" ? data[{json.dumps(VISIT_ID_PROPERTY)}] : null,
    attestationEnabled: data[{json.dumps(ATTESTATION_PROPERTY)}] !== false,
  }};
}}"""


def visit_id(raw_visit_id: str) -> str:
    encoded = base64.urlsafe_b64encode(raw_visit_id.encode()).decode().rstrip("=")
    return f"v1_{encoded}"


async def read_runtime_config(page: Any) -> dict[str, Any]:
    config = await page.evaluate(RUNTIME_SCRIPT)
    if not config.get("apiKey") or not config.get("rawVisitId"):
        config = _read_html_config(await page.content())
    if not config.get("apiKey") or not config.get("rawVisitId"):
        raise InvalidCookieSession(
            "Bootstrap response does not contain configured runtime markers"
        )
    config["visitId"] = visit_id(config["rawVisitId"])
    return config


def _read_html_config(html: str) -> dict[str, Any]:
    api_key = re.search(rf'"{re.escape(API_KEY_PROPERTY)}":"([^"\\]+)"', html)
    raw_visit_id = re.search(rf'"{re.escape(VISIT_ID_PROPERTY)}":"([^"\\]+)"', html)
    disabled = re.search(
        rf'"{re.escape(ATTESTATION_PROPERTY)}"\s*:\s*(?:false|"false")',
        html,
    )
    return {
        "apiKey": api_key.group(1) if api_key else None,
        "rawVisitId": raw_visit_id.group(1) if raw_visit_id else None,
        "attestationEnabled": disabled is None,
    }
