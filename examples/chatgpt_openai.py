"""دو turn متوالی با API OpenAI-compatible و conversation خودکار gateway."""

import json
import os
import urllib.error
import urllib.request


base_url = os.getenv("OPENAI_BASE_URL", "http://127.0.0.1:3346").rstrip("/")
model = os.getenv("CHATGPT_MODEL", "chatgpt/gpt-5.6-pro")
browser_id = os.getenv("CHATGPT_BROWSER_ID", "")
timeout = float(os.getenv("OPENAI_TIMEOUT", "240"))


def complete(messages):
    payload = {
        "model": model,
        "messages": messages,
    }
    if browser_id:
        payload["browser_id"] = browser_id
    request = urllib.request.Request(
        f"{base_url}/v1/chat/completions",
        data=json.dumps(payload).encode(),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        detail = error.read().decode(errors="replace")
        raise RuntimeError(f"Chat completion returned HTTP {error.code}: {detail}") from error


first_messages = [{"role": "user", "content": "Remember this code word: blue-orbit. Reply briefly."}]
first = complete(first_messages)
first_text = first["choices"][0]["message"]["content"]
print("turn 1:", first_text)

second_messages = first_messages + [
    {"role": "assistant", "content": first_text},
    {"role": "user", "content": "What was the code word?"},
]
second = complete(second_messages)

print("turn 2:", second["choices"][0]["message"]["content"])
