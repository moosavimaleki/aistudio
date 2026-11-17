"""Exploratory sequential direct-chat probe. Stops at the first error; no retries."""

import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import statistics
import time
from urllib.error import HTTPError
from urllib.request import Request, urlopen
import uuid


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", default="http://127.0.0.1:3350")
    parser.add_argument("--model", default="chatgpt/gpt-6-thinking-standard")
    parser.add_argument("--max-turns", type=int, default=100)
    parser.add_argument("--max-minutes", type=float, default=30)
    parser.add_argument("--report", type=Path, default=Path("/tmp/chatgpt-rate-limit-probe.json"))
    args = parser.parse_args()
    if args.max_turns < 1 or args.max_minutes <= 0:
        parser.error("limits must be positive")
    start = time.monotonic()
    report = {
        "started_at": datetime.now(timezone.utc).isoformat(),
        "url": args.url, "model": args.model, "concurrency": 1,
        "max_turns": args.max_turns, "max_minutes": args.max_minutes,
        "conversation_id": None, "requests": [], "stop_reason": "running",
        "rate_limit_observed": False,
        "pid": os.getpid(), "in_flight": None,
    }
    run_id = uuid.uuid4().hex[:8]

    def save():
        report["elapsed_seconds"] = round(time.monotonic() - start, 2)
        durations = [r["seconds"] for r in report["requests"] if r.get("ok")]
        report["successful_turns"] = len(durations)
        if durations:
            report["latency_seconds"] = {
                "median": round(statistics.median(durations), 2),
                "max": max(durations),
            }
        temporary = args.report.with_suffix(".tmp")
        temporary.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
        temporary.replace(args.report)

    save()
    try:
        for turn in range(1, args.max_turns + 1):
            remaining = args.max_minutes * 60 - (time.monotonic() - start)
            if remaining <= 0:
                report["stop_reason"] = "time_limit"
                break
            expected = f"ACK-{run_id}-{turn:03d}"
            body = {"model": args.model, "messages": [{
                "role": "user", "content": f"آزمون فنی شماره {turn}. فقط همین عبارت را بنویس: {expected}",
            }]}
            if report["conversation_id"]:
                body["conversation_id"] = report["conversation_id"]
            request = Request(args.url.rstrip("/") + "/v1/chat/completions",
                              data=json.dumps(body).encode(), headers={"Content-Type": "application/json"})
            request_start = time.monotonic()
            row = {"number": turn, "started_at": datetime.now(timezone.utc).isoformat()}
            report["in_flight"] = row.copy()
            save()
            try:
                with urlopen(request, timeout=min(330, remaining)) as response:
                    data = json.load(response)
                    row["http_status"] = response.status
                metadata = data["lab_metadata"]
                reply = data["choices"][0]["message"]["content"]
                if metadata["transport"] != "go-direct":
                    raise ValueError("unexpected transport")
                if report["conversation_id"] is None:
                    report["conversation_id"] = metadata["conversation_id"]
                if report["conversation_id"] != metadata["conversation_id"]:
                    raise ValueError("conversation ID changed")
                if expected not in reply:
                    raise ValueError("reply did not contain the expected acknowledgement")
                row.update(ok=True, reply=reply, usage=data.get("usage"),
                           timings_ms=metadata.get("timings_ms"),
                           parent_message_id=metadata.get("parent_message_id"))
            except HTTPError as error:
                # API error envelopes contain stage/status diagnostics, not auth headers.
                failure = error.read().decode("utf-8", errors="replace")
                row.update(ok=False, http_status=error.code, error=failure,
                           retry_after=error.headers.get("Retry-After"))
                report["rate_limit_observed"] = error.code == 429 or "429" in failure or "rate limit" in failure.lower()
                report["stop_reason"] = "rate_limit" if report["rate_limit_observed"] else "http_error"
            except Exception as error:
                row.update(ok=False, error=f"{type(error).__name__}: {error}")
                report["stop_reason"] = "client_or_transport_error"
            row["seconds"] = round(time.monotonic() - request_start, 2)
            report["requests"].append(row)
            report["in_flight"] = None
            save()
            print(json.dumps({"turn": turn, "ok": row["ok"], "seconds": row["seconds"],
                              "elapsed_seconds": report["elapsed_seconds"],
                              "stop_reason": report["stop_reason"],
                              "error": row.get("error")}, ensure_ascii=False), flush=True)
            if not row["ok"]:
                break
        else:
            report["stop_reason"] = "turn_limit"
    except KeyboardInterrupt:
        report["stop_reason"] = "interrupted"
    finally:
        save()
    print(json.dumps({k: report[k] for k in ("model", "successful_turns", "elapsed_seconds",
                                            "stop_reason", "rate_limit_observed")}), flush=True)


if __name__ == "__main__":
    main()
