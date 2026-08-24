"""تبدیل ترتیبی و قابل‌ادامهٔ ویدیوهای NCT به متن آموزشی فارسی.

هر خروجی کنار ویدیوی منبع با پسوند ``.nct.md`` نوشته می‌شود. وجود marker
پایانی یعنی فایل کامل است؛ بنابراین اجرای دوباره فقط ویدیوهای باقی‌مانده را
می‌فرستد. خروجی پس از دریافت پاسخ کامل به‌صورت atomic ذخیره می‌شود تا قطع
شدن process، فایل نیمه‌کاره را به‌عنوان نتیجهٔ کامل ثبت نکند.
"""

from __future__ import annotations

import argparse
import os
import sys
import tempfile
from datetime import UTC, datetime
from pathlib import Path

from vertex_client import generate_content, inline_data, text


DEFAULT_DIRECTORY = Path("/home/h-mousavi/Videos/NCT")
SUPPORTED_SUFFIXES = {".mkv", ".mp4", ".webm", ".mov", ".avi"}
COMPLETE_MARKER = "<!-- nct-transcription: complete -->"
PROMPT = """این ویدیوی آموزشی روش NCT/نودشماری را به متن مرجع دقیق تبدیل کن.

گفتار گوینده را بدون خلاصه‌سازی، با حفظ ترتیب و جزئیات، زیر بخش‌های
[صوت گوینده] بنویس. هرجا تصویر، نمودار، نوشته، حرکت روی چارت یا نمایش عملی
برای فهم آموزش لازم است، توضیح مستقل و دقیق زیر [توضیح تصویر] اضافه کن؛
چیزهای تزئینی را توضیح نده. خروجی فارسی، پیوسته و مناسب استخراج کانتکست کامل
روش NCT باشد. در صورت تغییر موضوع، timestamp تقریبی و عنوان کوتاه بخش را اضافه کن."""


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--directory",
        type=Path,
        default=Path(os.environ.get("NCT_VIDEO_DIR", DEFAULT_DIRECTORY)),
        help="پوشهٔ ویدیوها و خروجی‌ها (پیش‌فرض: %(default)s)",
    )
    parser.add_argument("--model", default="gemini-3.7-flash")
    parser.add_argument("--timeout", type=float, default=1800)
    parser.add_argument("--max-output-tokens", type=int, default=16000)
    parser.add_argument("--limit", type=int, default=0, help="حداکثر تعداد ویدیو؛ صفر یعنی همه")
    parser.add_argument("--force", action="store_true", help="خروجی‌های کامل را نیز دوباره تولید کن")
    parser.add_argument("--dry-run", action="store_true", help="فقط فهرست کارهای باقی‌مانده را نشان بده")
    return parser.parse_args()


def videos(directory: Path) -> list[Path]:
    return sorted(
        (path for path in directory.iterdir() if path.is_file() and path.suffix.lower() in SUPPORTED_SUFFIXES),
        key=lambda path: path.name.casefold(),
    )[50:]


def output_path(video: Path) -> Path:
    return video.with_name(f"{video.stem}.nct.md")


def complete(path: Path) -> bool:
    if not path.is_file():
        return False
    return COMPLETE_MARKER in path.read_text(encoding="utf-8", errors="replace")


def request_body(video: Path, max_output_tokens: int) -> dict:
    return {
        "contents": [{
            "role": "user",
            "parts": [
                {"text": PROMPT},
                {"inlineData": inline_data(video)},
            ],
        }],
        "generationConfig": {
            "thinkingConfig": {"levelEnum": 0},
            "maxOutputTokens": max_output_tokens,
        },
    }


def save_output(video: Path, model: str, transcription: str) -> Path:
    destination = output_path(video)
    created_at = datetime.now(UTC).isoformat(timespec="seconds")
    content = (
        f"# {video.stem}\n\n"
        f"- Source: `{video.name}`\n"
        f"- Model: `{model}`\n"
        f"- Generated at: `{created_at}`\n\n"
        f"{transcription.rstrip()}\n\n{COMPLETE_MARKER}\n"
    )
    with tempfile.NamedTemporaryFile(
        mode="w",
        encoding="utf-8",
        dir=destination.parent,
        prefix=f".{destination.name}.",
        suffix=".tmp",
        delete=False,
    ) as temporary:
        temporary.write(content)
        temporary_path = Path(temporary.name)
    temporary_path.replace(destination)
    return destination


def transcribe(video: Path, model: str, timeout: float, max_output_tokens: int) -> Path:
    response = generate_content(model, request_body(video, max_output_tokens), timeout=timeout)
    transcription = text(response)
    if not transcription.strip():
        raise RuntimeError("Gemini returned an empty transcription")
    return save_output(video, model, transcription)


def main() -> int:
    args = parse_args()
    directory = args.directory.expanduser().resolve()
    if not directory.is_dir():
        print(f"NCT directory does not exist: {directory}", file=sys.stderr)
        return 2

    pending = [
        video for video in videos(directory)
        if args.force or not complete(output_path(video))
    ]
    if args.limit > 0:
        pending = pending[:args.limit]
    if not pending:
        print("No pending NCT videos.")
        return 0

    print(f"Pending videos: {len(pending)}")
    if args.dry_run:
        for video in pending:
            print(f"- {video.name} -> {output_path(video).name}")
        return 0

    failures = 0
    for index, video in enumerate(pending, start=1):
        print(f"[{index}/{len(pending)}] {video.name}", flush=True)
        try:
            destination = transcribe(video, args.model, args.timeout, args.max_output_tokens)
        except Exception as error:  # اجرای بعدی دقیقاً از همین فایل دوباره ادامه می‌دهد.
            failures += 1
            print(f"FAILED: {video.name}: {error}", file=sys.stderr, flush=True)
            continue
        print(f"DONE: {destination}", flush=True)
    return 1 if failures else 0


if __name__ == "__main__":
    raise SystemExit(main())
