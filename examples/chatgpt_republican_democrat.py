"""دو conversation مستقل ChatGPT که پاسخ‌ها را به یکدیگر منتقل می‌کنند.

برای توقف حلقهٔ باز، Ctrl+C بزنید. پیش‌فرض فاصلهٔ ۲ ثانیه بین هر درخواست است.
این مثال برای آزمون state conversation است و گفت‌وگو را به سیاست عمومی محترمانه
محدود می‌کند؛ مخاطب یا رأی‌دهندهٔ واقعی هدف آن نیست.
"""

from __future__ import annotations

import argparse
import json
import os
import time
import urllib.error
import urllib.request
from dataclasses import dataclass


BASE_URL = os.getenv("OPENAI_BASE_URL", "http://127.0.0.1:3346").rstrip("/")
MODEL = os.getenv("CHATGPT_MODEL", "chatgpt/gpt-5.6-pro")
TIMEOUT = float(os.getenv("OPENAI_TIMEOUT", "240"))
DELAY_SECONDS = float(os.getenv("CHATGPT_DUAL_DELAY_SECONDS", "2"))
TURN_LIMIT = int(os.getenv("CHATGPT_DUAL_TURN_LIMIT", "0"))
DEFAULT_BROWSER_ID = os.getenv("CHATGPT_BROWSER_ID", "chatgpt12")


@dataclass
class Participant:
    name: str
    introduction: str
    browser_id: str
    conversation_id: str = ""
    parent_message_id: str = ""


def complete(participant: Participant, prompt: str) -> str:
    payload = {
        "model": MODEL,
        "messages": [{"role": "user", "content": prompt}],
    }
    browser_id = participant.browser_id or DEFAULT_BROWSER_ID
    if browser_id:
        payload["browser_id"] = browser_id
    if participant.conversation_id:
        payload["conversation_id"] = participant.conversation_id
        payload["parent_message_id"] = participant.parent_message_id

    request = urllib.request.Request(
        f"{BASE_URL}/v1/chat/completions",
        data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=TIMEOUT) as response:
            result = json.load(response)
    except urllib.error.HTTPError as error:
        detail = error.read().decode(errors="replace")
        raise RuntimeError(f"{participant.name} failed with HTTP {error.code}: {detail}") from error

    metadata = result["lab_metadata"]
    participant.conversation_id = metadata["conversation_id"]
    participant.parent_message_id = metadata["parent_message_id"]
    return result["choices"][0]["message"]["content"]


def introduction_prompt(role: str) -> str:
    return f"""You are a fictional {role} participant in a private policy discussion.
Introduce yourself in 120 words or fewer. State the values you bring to a respectful
discussion of public policy. Do not impersonate a real person, campaign, target voters,
or ask the other participant for personal information باید فارسی صحبت کنیم و موضع صحبت در مورد ایران است."""


def reply_prompt(participant: Participant, other_name: str, message: str) -> str:
    return f"""Continue your own private conversation as {participant.name}.
The latest message from {other_name} is below. Reply directly, respectfully, and in no
more than 180 words. Discuss policy reasoning and trade-offs; do not campaign, target
voters, or impersonate real people باید فارسی صحبت کنیم و موضع صحبت در مورد ایران است.

{other_name}: {message}"""


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--turn-limit",
        type=int,
        default=TURN_LIMIT,
        help="تعداد turnهای رفت‌وبرگشت؛ صفر یعنی بی‌نهایت (پیش‌فرض: %(default)s)",
    )
    parser.add_argument(
        "--delay-seconds",
        type=float,
        default=DELAY_SECONDS,
        help="فاصلهٔ بین درخواست‌ها (پیش‌فرض: %(default)s)",
    )
    parser.add_argument(
        "--republican-browser-id",
        default=os.getenv("CHATGPT_REPUBLICAN_BROWSER_ID", ""),
        help="browser profile جمهوری‌خواه؛ خالی یعنی profile پیش‌فرض",
    )
    parser.add_argument(
        "--democrat-browser-id",
        default=os.getenv("CHATGPT_DEMOCRAT_BROWSER_ID", ""),
        help="browser profile دموکرات؛ خالی یعنی profile پیش‌فرض",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    if args.turn_limit < 0:
        raise ValueError("--turn-limit must be zero or positive")
    if args.delay_seconds < 0:
        raise ValueError("--delay-seconds must be zero or positive")
    republican = Participant(
        name="Republican participant",
        introduction=introduction_prompt("Republican"),
        browser_id=args.republican_browser_id,
    )
    democrat = Participant(
        name="Democratic participant",
        introduction=introduction_prompt("Democratic"),
        browser_id=args.democrat_browser_id,
    )

    try:
        republican_message = complete(republican, republican.introduction)
        print(f"[Republican / introduction]\n{republican_message}\n", flush=True)
        time.sleep(args.delay_seconds)

        democrat_message = complete(democrat, democrat.introduction)
        print(f"[Democrat / introduction]\n{democrat_message}\n", flush=True)

        turn = 0
        while args.turn_limit == 0 or turn < args.turn_limit:
            turn += 1
            time.sleep(args.delay_seconds)
            republican_message = complete(
                republican,
                reply_prompt(republican, democrat.name, democrat_message),
            )
            print(f"[Turn {turn} / Republican]\n{republican_message}\n", flush=True)

            time.sleep(args.delay_seconds)
            democrat_message = complete(
                democrat,
                reply_prompt(democrat, republican.name, republican_message),
            )
            print(f"[Turn {turn} / Democrat]\n{democrat_message}\n", flush=True)
    except KeyboardInterrupt:
        print("\nStopped by user.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
