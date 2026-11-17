"""Use this project's APIs, never the benchmark website as an inference API."""

import json
import os
import time
from urllib.error import HTTPError
from urllib.request import Request, urlopen

try:
    from .. import vertex_client
except ImportError:  # python examples/youtube_benchmark.py
    import vertex_client


def complete(model: str, prompt: str, args, *, judge: bool = False) -> dict:
    started = time.monotonic()
    if model.startswith(("chatgpt/", "iran/")) or model == "chatgpt-web":
        body = {"model": model, "messages": [{"role": "user", "content": prompt}]}
        if args.browser_id:
            body["browser_id"] = args.browser_id
        base = os.environ["AISTUDIO_GENCONTENT_URL"].rstrip("/")
        request = Request(
            base + "/v1/chat/completions", data=json.dumps(body).encode(),
            headers={"Content-Type": "application/json"}, method="POST",
        )
        try:
            with urlopen(request, timeout=args.timeout) as response:
                raw = json.load(response)
        except HTTPError as error:
            raise RuntimeError(
                f"Chat completion HTTP {error.code}: {error.read().decode(errors='replace')}"
            ) from error
        output = raw["choices"][0]["message"]["content"]
        finish = raw["choices"][0].get("finish_reason")
        usage = raw.get("usage", {})
    else:
        config = vertex_client.thinking(args.judge_thinking if judge else args.thinking)
        config["maxOutputTokens"] = 4096 if judge else args.max_output_tokens
        body = {"contents": [{"role": "user", "parts": [{"text": prompt}]}],
                "generationConfig": config}
        raw = vertex_client.generate_content(model, body, timeout=args.timeout)
        candidate = raw.get("candidates", [{}])[0]
        parts = candidate.get("content", {}).get("parts", [])
        output = "".join(part.get("text", "") for part in parts if not part.get("thought"))
        finish = candidate.get("finishReason")
        usage = raw.get("usageMetadata", {})
    if not isinstance(output, str) or not output.strip():
        raise ValueError("API returned no visible script text")
    if finish in ("MAX_TOKENS", "length", 2):
        raise ValueError("API output was truncated; increase output budget before scoring")
    return {"text": output, "usage": usage, "finish_reason": finish,
            "seconds": round(time.monotonic() - started, 2), "response": raw}
