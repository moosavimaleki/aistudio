"""ارسال ۲۰۰ سؤال متوالی در یک conversation واقعی ChatGPT."""

import json
import os
import time
import urllib.error
import urllib.request
from pathlib import Path


base_url = os.getenv("OPENAI_BASE_URL", "http://127.0.0.1:3346").rstrip("/")
model = os.getenv("CHATGPT_MODEL", "chatgpt/gpt-5.6-pro")
browser_id = os.getenv("CHATGPT_BROWSER_ID", "")
delay_seconds = float(os.getenv("CHATGPT_DELAY_SECONDS", "2"))
timeout = float(os.getenv("OPENAI_TIMEOUT", "240"))
questions_file = Path(__file__).with_name("assets") / "chatgpt_200_questions.fa.txt"


def complete(prompt, conversation_id="", parent_message_id=""):
    payload = {
        "model": model,
        "messages": [{"role": "user", "content": prompt}],
    }
    if browser_id:
        payload["browser_id"] = browser_id
    if conversation_id:
        payload["conversation_id"] = conversation_id
        payload["parent_message_id"] = parent_message_id
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
        raise RuntimeError(f"Question failed with HTTP {error.code}: {detail}") from error


questions = [
    line.strip()
    for line in questions_file.read_text(encoding="utf-8").splitlines()
    if line.strip()
]
question_count = int(os.getenv("CHATGPT_QUESTION_COUNT", str(len(questions))))
if question_count < 1 or question_count > len(questions):
    raise ValueError(f"CHATGPT_QUESTION_COUNT must be between 1 and {len(questions)}")


conversation_id = ""
parent_message_id = ""

try:
    for number in range(1, question_count + 1):
        if number > 1:
            time.sleep(delay_seconds)
        result = complete(questions[number - 1], conversation_id, parent_message_id)
        metadata = result["lab_metadata"]
        browser_id = metadata["browser_id"]
        conversation_id = metadata["conversation_id"]
        parent_message_id = metadata["parent_message_id"]
        answer = result["choices"][0]["message"]["content"]
        print(f"[{number:03d}/{question_count}] {answer}", flush=True)
except KeyboardInterrupt:
    print("\nStopped by user.")
