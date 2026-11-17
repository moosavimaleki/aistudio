"""Small, auditable local checks; no claim of official benchmark scoring."""

import json
import re

from .prompts import RUBRIC


def script_checks(script: str, target: int) -> dict:
    spoken = re.sub(r"\[SHOW:.*?\]", " ", script, flags=re.DOTALL | re.IGNORECASE)
    words = re.findall(r"[^\W_]+(?:['’\u200c][^\W_]+)*", spoken, flags=re.UNICODE)
    return {
        "spoken_words_approx": len(words),
        "target_words": target,
        "length_ratio": round(len(words) / target, 3),
        "visual_cues": len(re.findall(r"\[SHOW:", script, flags=re.IGNORECASE)),
    }


def parse_judgment(raw: str, script: str) -> dict:
    value = raw.strip()
    if value.startswith("```json\n") and value.endswith("```"):
        value = value[8:-3].strip()
    result = json.loads(value)
    criteria = result.get("criteria") if isinstance(result, dict) else None
    if not isinstance(criteria, dict) or set(criteria) != set(RUBRIC):
        raise ValueError("Judge must return exactly the five local rubric criteria")
    for name, entry in criteria.items():
        if not isinstance(entry, dict):
            raise ValueError(f"Invalid entry: {name}")
        score, quotes = entry.get("score"), entry.get("evidence")
        if type(score) is not int or not 0 <= score <= 4:
            raise ValueError(f"Score for {name} must be an integer from 0 to 4")
        if not isinstance(entry.get("reason"), str) or not entry["reason"].strip():
            raise ValueError(f"Missing reason for {name}")
        if not isinstance(quotes, list) or any(
            not isinstance(quote, str) or not quote.strip() or quote not in script
            for quote in quotes
        ):
            raise ValueError(f"Evidence for {name} must quote the candidate exactly")
        if score > 0 and not quotes:
            raise ValueError(f"Positive score for {name} requires evidence")
    result["youtube_best_practices_local"] = sum(
        entry["score"] for entry in criteria.values()
    ) * 5
    return result
