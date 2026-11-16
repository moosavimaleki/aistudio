"""Opt-in live test: 20 HTTP chat turns, stable ID, and first-turn recall.

Start go run ./cmd/chatgpt-direct, then run this file explicitly.
This sends real prompts; it is not part of the offline unit test suite.
"""

import argparse
import json
from pathlib import Path
import time
from urllib.error import HTTPError
from urllib.request import Request, urlopen
import uuid


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", default="http://127.0.0.1:8788")
    parser.add_argument("--turns", type=int, default=20)
    parser.add_argument("--report", type=Path)
    args = parser.parse_args()
    if args.turns < 2:
        parser.error("--turns must be at least 2")
    secret = f"memory-{uuid.uuid4().hex[:12]}"
    report = {"url": args.url, "conversation_id": None, "turns": [], "passed": False}

    def save():
        if args.report:
            args.report.parent.mkdir(parents=True, exist_ok=True)
            args.report.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")

    try:
        for number in range(1, args.turns + 1):
            if number == 1:
                expected = "READY"
                prompt = f"سلام. رمز این گفتگو {secret} است. آن را به خاطر بسپار. فقط بنویس READY"
            elif number == args.turns:
                expected = secret
                prompt = "رمز گفتگو که در اولین پیام گفتم چه بود؟ فقط همان رمز را بنویس."
            else:
                expected = f"OK-{number:02d}"
                greeting = ("خوبی؟", "چطوری؟", "سلام دوباره.")[number % 3]
                prompt = f"{greeting} مرحله {number} از {args.turns} آزمون است. فقط بنویس {expected}"
            payload = {"message": prompt, "conversation_id": report["conversation_id"]}
            request = Request(args.url.rstrip("/") + "/chat",
                              data=json.dumps(payload).encode(),
                              headers={"Content-Type": "application/json"})
            start = time.monotonic()
            with urlopen(request, timeout=330) as response:
                data = json.load(response)
            conversation_id = data["conversation_id"]
            if report["conversation_id"] is None:
                report["conversation_id"] = conversation_id
            assert conversation_id == report["conversation_id"], "conversation ID changed"
            assert expected in data["reply"], f"unexpected reply at turn {number}: {data['reply']!r}"
            report["turns"].append({"number": number, "reply": data["reply"],
                                    "seconds": round(time.monotonic() - start, 2)})
            print(f"{number}/{args.turns} OK: {data['reply']}", flush=True)
            save()
        report["passed"] = True
        save()
        print(f"PASS: {args.turns} turns; first-turn memory recalled; conversation {report['conversation_id']}")
    except Exception as error:
        report["error"] = error.read().decode() if isinstance(error, HTTPError) else str(error)
        save()
        raise


if __name__ == "__main__":
    main()
