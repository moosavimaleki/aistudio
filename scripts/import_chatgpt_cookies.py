#!/usr/bin/env python3
"""Export every healthy local Chrome ChatGPT session as Netscape cookies."""

from __future__ import annotations

import argparse
import os
import re
from pathlib import Path

if __package__:
    from .chatgpt_cookie_import import (
        discover_profiles,
        extract_chatgpt_cookies,
        validate_session,
        write_netscape,
    )
else:
    from chatgpt_cookie_import import (
        discover_profiles,
        extract_chatgpt_cookies,
        validate_session,
        write_netscape,
    )


PROJECT_ROOT = Path(__file__).resolve().parents[1]
GENERATED_EXPORT = re.compile(r"^chatgpt-\d{2}\.txt$")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Import active chatgpt.com sessions from local Chrome profiles.",
    )
    parser.add_argument(
        "--chrome-root",
        action="append",
        type=Path,
        help="Chrome user-data root; may be repeated. Standard Linux roots are scanned by default.",
    )
    parser.add_argument(
        "--output-dir",
        type=Path,
        default=PROJECT_ROOT / "CHATGPT_COOKIES",
        help="Netscape output directory (default: project CHATGPT_COOKIES).",
    )
    parser.add_argument(
        "--proxy",
        default=(
            os.getenv("CHATGPT_PROXY_URL")
            or os.getenv("AISTUDIO_PROXY_URL")
            or os.getenv("LAB_PROXY_URL")
            or "http://127.0.0.1:10811"
        ),
        help="Proxy used only to validate each ChatGPT session.",
    )
    parser.add_argument("--timeout", type=float, default=20, help="Session validation timeout in seconds.")
    parser.add_argument("--replace", action="store_true", help="Atomically replace previously generated files.")
    parser.add_argument("--dry-run", action="store_true", help="Validate profiles without writing cookie files.")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    profiles = discover_profiles(args.chrome_root)
    if not profiles:
        print("No local Chrome profile with a Cookies database was found.")
        return 1

    healthy = []
    for profile in profiles:
        try:
            cookies = extract_chatgpt_cookies(profile)
            if not list(cookies):
                print(f"SKIP {profile.label}: no unexpired chatgpt.com cookies")
                continue
            active, reason = validate_session(cookies, args.proxy, args.timeout)
            if not active:
                print(f"SKIP {profile.label}: {reason}")
                continue
            healthy.append((profile, cookies))
            if args.dry_run:
                print(f"OK   {profile.label}: active session")
        except FileExistsError as error:
            print(f"SKIP {profile.label}: {error}")
        except Exception as error:  # Keep one broken Chrome profile from hiding the others.
            print(f"SKIP {profile.label}: {type(error).__name__}; profile processing failed")

    exported = len(healthy) if args.dry_run else 0
    if not args.dry_run:
        output_names: set[str] = set()
        for index, (profile, cookies) in enumerate(healthy, start=1):
            output = args.output_dir / f"chatgpt-{index:02d}.txt"
            try:
                write_netscape(output, cookies, args.replace)
            except FileExistsError as error:
                print(f"SKIP {profile.label}: {error}")
                continue
            output_names.add(output.name)
            exported += 1
            print(f"SAVE {profile.label}: {output.name}")
        if args.replace and exported == len(healthy):
            remove_stale_exports(args.output_dir, output_names)

    action = "validated" if args.dry_run else "exported"
    print(f"{action} {exported} healthy session(s) from {len(profiles)} Chrome profile(s).")
    return 0 if exported else 1


def remove_stale_exports(output_dir: Path, current_names: set[str]) -> None:
    """Remove only obsolete numbered files that this importer generated."""
    if not output_dir.is_dir():
        return
    for path in output_dir.iterdir():
        if path.is_file() and GENERATED_EXPORT.fullmatch(path.name) and path.name not in current_names:
            path.unlink()


if __name__ == "__main__":
    raise SystemExit(main())
