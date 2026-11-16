"""Live main-server test: 20 turns through /v1/chat/completions, no retries."""
import argparse
import json
from pathlib import Path
import time
from urllib.request import Request, urlopen
from urllib.error import HTTPError
import uuid


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", default="http://127.0.0.1:8000")
    parser.add_argument("--report", type=Path, default=Path("/tmp/chatgpt-integrated-20.json"))
    args = parser.parse_args()
    secret = "remember-" + uuid.uuid4().hex[:12]
    report = {"conversation_id": None, "turns": [], "passed": False}
    try:
        for turn in range(1, 21):
            expected = "READY" if turn == 1 else f"OK-{turn:02d}"
            prompt = f"سلام. فقط بنویس {expected}"
            if turn == 1:
                prompt = f"رمز این گفتگو {secret} است. به خاطر بسپار. فقط بنویس READY"
            if turn == 20:
                prompt, expected = "رمز گفتگو را که در اولین پیام گفتم بنویس. فقط همان رمز.", secret
            body = {"model": "chatgpt-web", "messages": [{"role": "user", "content": prompt}]}
            if report["conversation_id"]:
                body["conversation_id"] = report["conversation_id"]
            request = Request(args.url.rstrip("/") + "/v1/chat/completions",
                              data=json.dumps(body).encode(), headers={"Content-Type": "application/json"})
            start = time.monotonic()
            with urlopen(request, timeout=330) as response:
                result = json.load(response)
            metadata = result["lab_metadata"]
            reply = result["choices"][0]["message"]["content"]
            assert metadata["transport"] == "go-direct", metadata
            if report["conversation_id"] is None:
                report["conversation_id"] = metadata["conversation_id"]
            assert metadata["conversation_id"] == report["conversation_id"]
            assert expected in reply, reply
            report["turns"].append({"number": turn, "reply": reply,
                                    "parent_message_id": metadata["parent_message_id"],
                                    "seconds": round(time.monotonic()-start, 2)})
            args.report.write_text(json.dumps(report, ensure_ascii=False, indent=2)+"\n")
            print(f"{turn}/20 OK: {reply}", flush=True)
        report["passed"] = True
    except HTTPError as error:
        report["error"] = {"status": error.code, "body": error.read().decode()}
        print(json.dumps(report["error"], ensure_ascii=False), flush=True)
        raise
    finally:
        args.report.write_text(json.dumps(report, ensure_ascii=False, indent=2)+"\n")


if __name__ == "__main__":
    main()
