#!/usr/bin/env python3
"""Measure first non-empty content chunk and completion time, not just HTTP headers."""
import argparse
import json
import os
import time
import urllib.error
import urllib.request


def measure(args):
    payload = {"model": args.model, "stream": True, "messages": [
        {"role": "user", "content": args.prompt}]}
    if args.browser_id:
        payload["browser_id"] = args.browser_id
    headers = {"Content-Type": "application/json"}
    if token := os.getenv("OPENAI_API_KEY"):
        headers["Authorization"] = f"Bearer {token}"
    request = urllib.request.Request(args.base_url.rstrip("/") + "/v1/chat/completions",
                                     data=json.dumps(payload).encode(), headers=headers)
    started = time.monotonic()
    first = None
    chunks = 0
    text = ""
    with urllib.request.urlopen(request, timeout=args.timeout) as response:
        headers_seconds = time.monotonic() - started
        mode = response.headers.get("X-Lab-Streaming-Mode", "unspecified")
        timings = json.loads(response.headers.get("X-Lab-Timings", "{}"))
        request_id = response.headers.get("X-Lab-Request-ID")
        for line in response:
            if not line.startswith(b"data: "):
                continue
            data = line[6:].strip()
            if data == b"[DONE]":
                break
            event = json.loads(data)
            if "error" in event:
                raise RuntimeError(event["error"])
            for choice in event.get("choices", []):
                content = choice.get("delta", {}).get("content")
                if content:
                    first = first if first is not None else time.monotonic() - started
                    chunks += 1
                    text += content
    return {"ok": bool(text), "streaming_mode": mode, "content_chunks": chunks,
            "timings_ms": timings, "request_id": request_id,
            "headers_seconds": round(headers_seconds, 3),
            "first_content_seconds": round(first, 3) if first is not None else None,
            "total_seconds": round(time.monotonic() - started, 3), "characters": len(text)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", default="http://127.0.0.1:3346")
    parser.add_argument("--model", default="chatgpt/gpt-5.6-thinking-off")
    parser.add_argument("--browser-id", default="", help="Pin one profile for a comparable warm-page test")
    parser.add_argument("--prompt", default="In three short sentences explain how rain forms.")
    parser.add_argument("--requests", type=int, default=3)
    parser.add_argument("--timeout", type=int, default=180)
    args = parser.parse_args()
    if min(args.requests, args.timeout) < 1:
        parser.error("requests and timeout must be positive")
    failures = 0
    for number in range(1, args.requests + 1):
        started = time.monotonic()
        try:
            result = measure(args)
        except urllib.error.HTTPError as error:
            result = {"ok": False, "status": error.code,
                      "timings_ms": json.loads(error.headers.get("X-Lab-Timings", "{}")),
                      "request_id": error.headers.get("X-Lab-Request-ID"),
                      "error": error.read(2048).decode(errors="replace")}
        except (OSError, ValueError, RuntimeError) as error:
            result = {"ok": False, "error": str(error)}
        failures += not result["ok"]
        result.update(number=number, elapsed_seconds=round(time.monotonic() - started, 3))
        print(json.dumps(result, ensure_ascii=False), flush=True)
    return int(failures > 0)


if __name__ == "__main__":
    raise SystemExit(main())
