"""تولید یک تصویر با صفحهٔ واقعی ChatGPT و ذخیرهٔ آن در فایل محلی."""

import base64
import json
import os
from pathlib import Path
import urllib.request


base_url = os.getenv("OPENAI_BASE_URL", "http://127.0.0.1:3346").rstrip("/")
prompt = os.getenv(
    "CHATGPT_IMAGE_PROMPT",
    "A minimal flat illustration of a blue circle on a white background.",
)
payload = {
    "model": "chatgpt-web",
    "prompt": prompt,
    "response_format": "b64_json",
    "browser_id": os.getenv("CHATGPT_BROWSER_ID", "chatgpt"),
}
request = urllib.request.Request(
    f"{base_url}/v1/images/generations",
    data=json.dumps(payload).encode(),
    headers={"Content-Type": "application/json"},
    method="POST",
)
with urllib.request.urlopen(request, timeout=float(os.getenv("OPENAI_TIMEOUT", "240"))) as response:
    result = json.load(response)

image_data = result["data"][0]["b64_json"]
mime_type = result.get("lab_metadata", {}).get("mime_types", [""])[0]
extension = {"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}.get(
    mime_type, ".img"
)
output_path = Path(os.getenv("CHATGPT_IMAGE_OUTPUT", f"chatgpt-image{extension}"))
output_path.parent.mkdir(parents=True, exist_ok=True)
output_path.write_bytes(base64.b64decode(image_data, validate=True))
print(output_path.resolve())
