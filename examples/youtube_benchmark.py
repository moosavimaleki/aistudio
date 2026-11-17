"""Run an independent YouTube practices evaluation against the local gateway."""

import argparse
import json
import os
from pathlib import Path

try:
    from .youtube_eval.runner import run
except ImportError:
    from youtube_eval.runner import run


def parse_args():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--model", default="gemini-4-argon")
    parser.add_argument("--base-url", default=os.getenv(
        "AISTUDIO_GENCONTENT_URL", "http://127.0.0.1:3346"))
    parser.add_argument("--judge-model", default="", help="Optional, preferably a different model family")
    parser.add_argument("--browser-id", default="", help="Only for ChatGPT requests; optional")
    parser.add_argument("--thinking", type=int, choices=(1, 2, 3), default=1,
                        help="Gemini: 1=LOW, 2=MEDIUM, 3=HIGH")
    parser.add_argument("--judge-thinking", type=int, choices=(1, 2, 3), default=2,
                        help="Fixed Gemini judge level, independent of writer thinking")
    parser.add_argument("--language", choices=("English", "Persian"), default="English")
    parser.add_argument("--cases", type=Path, default=Path(__file__).with_name("youtube_eval") / "cases.json")
    parser.add_argument("--case", action="append", default=[], help="Case ID; repeat to select multiple")
    parser.add_argument("--drafts", type=int, default=1)
    parser.add_argument("--max-output-tokens", type=int, default=8192)
    parser.add_argument("--timeout", type=float, default=240)
    parser.add_argument("--delay", type=float, default=2)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    if args.drafts < 1 or args.max_output_tokens < 1 or args.timeout <= 0 or args.delay < 0:
        parser.error("drafts, max-output-tokens and timeout must be positive; delay must be nonnegative")
    return args


def load_cases(path: Path, selected: list[str]) -> list[dict]:
    cases = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(cases, list) or not cases:
        raise ValueError("Cases must be a nonempty JSON array")
    seen = set()
    for case in cases:
        if not isinstance(case, dict):
            raise ValueError("Each case must be a JSON object")
        name = case.get("id", "")
        if not isinstance(name, str) or not name or any(
            char not in "abcdefghijklmnopqrstuvwxyz0123456789-_" for char in name
        ):
            raise ValueError("Case IDs must be lowercase alphanumeric slugs")
        if name in seen:
            raise ValueError(f"Duplicate case ID: {name}")
        seen.add(name)
        if type(case.get("target_words")) is not int or case["target_words"] <= 0:
            raise ValueError(f"Invalid target_words: {name}")
        if not all(isinstance(case.get(key), str) and case[key].strip() for key in ("title", "brief")):
            raise ValueError(f"Missing title/brief: {name}")
        if not isinstance(case.get("research"), list) or not case["research"] or not all(
            isinstance(fact, str) and fact.strip() for fact in case["research"]
        ):
            raise ValueError(f"Research must be a nonempty list of facts: {name}")
    if set(selected) - seen:
        raise ValueError(f"Unknown cases: {sorted(set(selected) - seen)}")
    return [case for case in cases if not selected or case["id"] in selected]


def main():
    args = parse_args()
    os.environ["AISTUDIO_GENCONTENT_URL"] = args.base_url
    try:
        cases = load_cases(args.cases, args.case)
        print(f"Independent evaluation: {len(cases) * args.drafts} drafts; NOT official ToneBench.")
        if args.judge_model == args.model:
            print("WARNING: self-judging can bias scores. Use an independent judge for comparisons.")
        return run(args, cases)
    except (ValueError, OSError) as error:
        print(f"ERROR: {error}")
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
