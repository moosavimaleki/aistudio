"""ترجمهٔ قابل‌ادامهٔ کتاب AI Powered Search از Markdown انگلیسی به فارسی.

یک conversation ریشه با system instruction ساخته می‌شود. تمام فایل‌های بعدی
به همان parent ریشه وصل می‌شوند؛ بنابراین ترجمه‌ها sibling هستند و context
فصل‌های قبلی به درخواست بعدی افزوده نمی‌شود.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import sys
import tempfile
import urllib.error
import urllib.request
from dataclasses import asdict, dataclass
from pathlib import Path


SOURCE_DIRECTORY = Path("/home/h-mousavi/Projects/Hamed/books/books/ai-powered-search-v9/en")
TARGET_DIRECTORY = Path("/home/h-mousavi/Projects/Hamed/books/books/ai-powered-search-v9/fa")
DEFAULT_BASE_URL = "http://127.0.0.1:3346"
DEFAULT_MODEL = "chatgpt/gpt-5.6-pro"
STATE_FILENAME = ".translation-session.json"
COMPLETE_PREFIX = "<!-- ai-powered-search-translation: complete sha256="

SYSTEM_INSTRUCTION = """You are a meticulous technical book translator.
Translate English Markdown to fluent, accurate Persian. Preserve the Markdown structure,
headings, lists, tables, links, image references, HTML, citations, code fences, inline
code, commands, paths, API names, and identifiers. Do not summarize, omit, add commentary,
or translate code. Output only the translated Markdown."""


@dataclass
class Session:
    conversation_id: str
    root_parent_message_id: str
    model: str
    browser_id: str


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, default=SOURCE_DIRECTORY)
    parser.add_argument("--target", type=Path, default=TARGET_DIRECTORY)
    parser.add_argument("--base-url", default=os.getenv("OPENAI_BASE_URL", DEFAULT_BASE_URL))
    parser.add_argument("--model", default=os.getenv("CHATGPT_MODEL", DEFAULT_MODEL))
    parser.add_argument("--browser-id", default=os.getenv("CHATGPT_BROWSER_ID", "chatgpt10"))
    parser.add_argument("--timeout", type=float, default=float(os.getenv("OPENAI_TIMEOUT", "600")))
    parser.add_argument("--limit", type=int, default=0, help="تعداد فایل؛ صفر یعنی همه")
    parser.add_argument("--force", action="store_true", help="ترجمه‌های کامل را هم دوباره بساز")
    parser.add_argument("--reset-session", action="store_true", help="conversation ریشهٔ تازه بساز")
    parser.add_argument("--dry-run", action="store_true", help="فقط فایل‌های باقی‌مانده را نمایش بده")
    return parser.parse_args()


def natural_key(path: Path, root: Path) -> tuple[tuple[int, str | int], ...]:
    parts = re.split(r"(\d+)", path.relative_to(root).as_posix().casefold())
    return tuple((0, int(part)) if part.isdigit() else (1, part) for part in parts)


def source_files(directory: Path) -> list[Path]:
    return sorted(directory.rglob("*.md"), key=lambda path: natural_key(path, directory))


def destination_for(source: Path, source_root: Path, target_root: Path) -> Path:
    return target_root / source.relative_to(source_root)


def source_digest(source: Path) -> str:
    return hashlib.sha256(source.read_bytes()).hexdigest()


def is_complete(destination: Path, digest: str) -> bool:
    if not destination.is_file():
        return False
    return f"{COMPLETE_PREFIX}{digest} -->" in destination.read_text(encoding="utf-8", errors="replace")


def write_atomic(destination: Path, content: str) -> None:
    destination.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(
        mode="w", encoding="utf-8", dir=destination.parent,
        prefix=f".{destination.name}.", suffix=".tmp", delete=False,
    ) as temporary:
        temporary.write(content)
        temporary_path = Path(temporary.name)
    temporary_path.replace(destination)


def state_path(target: Path) -> Path:
    return target / STATE_FILENAME


def load_session(target: Path) -> Session | None:
    path = state_path(target)
    if not path.is_file():
        return None
    try:
        return Session(**json.loads(path.read_text(encoding="utf-8")))
    except (json.JSONDecodeError, TypeError, ValueError) as error:
        raise RuntimeError(f"Invalid translation session state: {path}") from error


def save_session(target: Path, session: Session) -> None:
    write_atomic(state_path(target), json.dumps(asdict(session), ensure_ascii=False, indent=2) + "\n")


def complete(
    base_url: str,
    timeout: float,
    model: str,
    browser_id: str,
    messages: list[dict[str, str]],
    session: Session | None,
) -> tuple[str, dict[str, str]]:
    payload: dict[str, object] = {
        "model": model,
        "browser_id": browser_id,
        "messages": messages,
    }
    if session:
        payload["conversation_id"] = session.conversation_id
        # هر فایل فرزند مستقیم instruction ریشه است، نه ترجمهٔ قبلی.
        payload["parent_message_id"] = session.root_parent_message_id
    request = urllib.request.Request(
        f"{base_url.rstrip('/')}/v1/chat/completions",
        data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            result = json.load(response)
    except urllib.error.HTTPError as error:
        detail = error.read().decode(errors="replace")
        raise RuntimeError(f"Translation request failed with HTTP {error.code}: {detail}") from error
    try:
        return result["choices"][0]["message"]["content"], result["lab_metadata"]
    except (KeyError, IndexError, TypeError) as error:
        raise RuntimeError("Translation response has no text or conversation metadata") from error


def create_session(base_url: str, timeout: float, model: str, browser_id: str) -> Session:
    messages = [
        {"role": "system", "content": SYSTEM_INSTRUCTION},
        {"role": "user", "content": "Acknowledge the translation instructions. Reply only READY."},
    ]
    _, metadata = complete(base_url, timeout, model, browser_id, messages, None)
    conversation_id = str(metadata.get("conversation_id", ""))
    parent_message_id = str(metadata.get("parent_message_id", ""))
    if not conversation_id or not parent_message_id:
        raise RuntimeError("Root instruction did not create a usable ChatGPT conversation")
    return Session(conversation_id, parent_message_id, model, browser_id)


def translation_prompt(source: Path, content: str) -> str:
    return f"""Translate the following Markdown file into Persian according to the root instruction.
Return only the translated Markdown, without a preface or explanation.

Source file: {source.name}

<source-markdown>
{content}
</source-markdown>"""


def main() -> int:
    args = parse_args()
    source_root = args.source.expanduser().resolve()
    target_root = args.target.expanduser().resolve()
    if not source_root.is_dir():
        print(f"Source directory does not exist: {source_root}", file=sys.stderr)
        return 2
    if args.limit < 0:
        print("--limit must be zero or positive", file=sys.stderr)
        return 2

    pending: list[tuple[Path, str]] = []
    for source in source_files(source_root):
        digest = source_digest(source)
        destination = destination_for(source, source_root, target_root)
        if args.force or not is_complete(destination, digest):
            pending.append((source, digest))
    if args.limit:
        pending = pending[:args.limit]
    print(f"Pending files: {len(pending)}")
    if args.dry_run:
        for source, _ in pending:
            print(f"- {source.relative_to(source_root)}")
        return 0
    if not pending:
        return 0

    session = None if args.reset_session else load_session(target_root)
    if session is None:
        print("Creating root translation conversation...", flush=True)
        session = create_session(args.base_url, args.timeout, args.model, args.browser_id)
        save_session(target_root, session)
    elif session.model != args.model or session.browser_id != args.browser_id:
        print("Session model/profile differs; use --reset-session to create a new root.", file=sys.stderr)
        return 2

    try:
        for index, (source, digest) in enumerate(pending, start=1):
            print(f"[{index}/{len(pending)}] {source.relative_to(source_root)}", flush=True)
            translated, _ = complete(
                args.base_url,
                args.timeout,
                session.model,
                session.browser_id,
                [{"role": "user", "content": translation_prompt(source, source.read_text(encoding="utf-8"))}],
                session,
            )
            if not translated.strip():
                raise RuntimeError(f"Empty translation: {source}")
            destination = destination_for(source, source_root, target_root)
            write_atomic(destination, translated.rstrip() + "\n\n" + f"{COMPLETE_PREFIX}{digest} -->\n")
            print(f"DONE: {destination}", flush=True)
    except KeyboardInterrupt:
        print("\nStopped. Re-run the same command to continue.")
        return 130
    except Exception as error:
        print(f"FAILED: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
