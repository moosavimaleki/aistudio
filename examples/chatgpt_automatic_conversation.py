"""Conversation خودکار ChatGPT با full OpenAI messages و بدون شناسه‌های داخلی."""

from __future__ import annotations

import json
import os
from urllib.request import Request, urlopen


BASE_URL = os.getenv("OPENAI_BASE_URL", "http://127.0.0.1:3346").rstrip("/")
MODEL = os.getenv("CHATGPT_MODEL", "chatgpt/gpt-5.6")
BROWSER_ID = os.getenv("CHATGPT_BROWSER_ID", "")
TIMEOUT = float(os.getenv("OPENAI_TIMEOUT", "180"))


def complete(messages: list[dict[str, str]]) -> dict:
    payload: dict[str, object] = {"model": MODEL, "messages": messages}
    if BROWSER_ID:
        payload["browser_id"] = BROWSER_ID
    request = Request(
        f"{BASE_URL}/v1/chat/completions",
        data=json.dumps(payload).encode(),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urlopen(request, timeout=TIMEOUT) as response:
        return json.load(response)


def answer(response: dict) -> str:
    return response["choices"][0]["message"]["content"]


system = {"role": "system", "content": "Answer with one word only."}
iran = {"role": "user", "content": "What is the capital of Iran?"}
first = complete([system, iran])
print("branch 1:", answer(first), first["lab_metadata"]["conversation_id"])

usa = {"role": "user", "content": "What is the capital of the United States?"}
second = complete([system, usa])
print("branch 2:", answer(second), second["lab_metadata"]["conversation_id"])

continuation = complete([
    system,
    iran,
    {"role": "assistant", "content": answer(first)},
    {"role": "user", "content": "What country is it in?"},
])
print("continuation:", answer(continuation), continuation["lab_metadata"]["conversation_id"])
