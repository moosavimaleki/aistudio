#!/usr/bin/env python3
"""Small direct-API concurrency smoke test; exits nonzero on any failure."""
import argparse
import concurrent.futures
import json
import time
import urllib.error
import urllib.request


def complete(base, model, number, timeout):
    started = time.monotonic()
    payload = {"model": model, "messages": [{"role": "user", "content":
        f"Translate to Persian, one sentence only: Add {number} spoons of water and stir."}], "stream": False}
    request = urllib.request.Request(base.rstrip("/") + "/v1/chat/completions",
        data=json.dumps(payload).encode(), headers={"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            result = json.load(response)
        text = result["choices"][0]["message"]["content"]
        return {"number": number, "ok": bool(text), "seconds": round(time.monotonic()-started, 2),
                "browser": result.get("lab_metadata", {}).get("browser_id"), "text": text}
    except urllib.error.HTTPError as error:
        detail = error.read(4096).decode(errors="replace")
        return {"number": number, "ok": False, "seconds": round(time.monotonic()-started, 2),
                "status": error.code, "error": detail}
    except (OSError, ValueError, KeyError, IndexError) as error:
        return {"number": number, "ok": False, "seconds": round(time.monotonic()-started, 2),
                "error": str(error)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", default="http://127.0.0.1:3346")
    parser.add_argument("--model", default="chatgpt/gpt-5.6-thinking-extended")
    parser.add_argument("--requests", type=int, default=6)
    parser.add_argument("--concurrency", type=int, default=2)
    parser.add_argument("--timeout", type=int, default=180)
    args = parser.parse_args()
    if min(args.requests, args.concurrency, args.timeout) < 1:
        parser.error("counts and timeout must be positive")
    failures = 0
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.concurrency) as pool:
        jobs = [pool.submit(complete, args.base_url, args.model, n, args.timeout)
                for n in range(1, args.requests+1)]
        for job in concurrent.futures.as_completed(jobs):
            result = job.result()
            failures += not result["ok"]
            print(json.dumps(result, ensure_ascii=False), flush=True)
    print(f"passed={args.requests-failures} failed={failures}")
    return int(failures > 0)


if __name__ == "__main__":
    raise SystemExit(main())
