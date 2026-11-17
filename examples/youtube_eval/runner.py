"""Sequential, resumable runs with explicit configuration and auditable artifacts."""

import hashlib
import json
import statistics
import time

from .prompts import judging_prompt, writing_prompt
from .scoring import parse_judgment, script_checks
from .transport import complete


def write_json(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def summarize(results: list[dict]) -> dict:
    scores = [row["judgment"]["youtube_best_practices_local"] for row in results
              if row.get("judgment") and not row.get("error")]
    return {
        "label": "Independent YouTube practices evaluation; NOT official ToneBench",
        "attempts": len(results),
        "errors": sum(bool(row.get("error")) for row in results),
        "judged": len(scores),
        "youtube_best_practices_local_mean": statistics.mean(scores) if scores else None,
        "youtube_best_practices_local_stdev": statistics.stdev(scores) if len(scores) > 1 else None,
        "results": results,
    }


def run(args, cases: list[dict]) -> int:
    args.output.mkdir(parents=True, exist_ok=True)
    identity = {key: value for key, value in vars(args).items()
                if key not in ("output", "case", "cases", "delay", "timeout")}
    # Includes the prompts so a changed rubric/style cannot silently reuse older scores.
    identity["suite_sha256"] = hashlib.sha256(json.dumps([
        cases, writing_prompt(cases[0], args.language),
        judging_prompt(cases[0], "", args.language),
    ], sort_keys=True, ensure_ascii=False).encode()).hexdigest()
    config_path = args.output / "config.json"
    if config_path.exists() and json.loads(config_path.read_text()) != identity:
        raise ValueError("Output directory belongs to different settings; choose a new --output")
    write_json(config_path, identity)
    results = []
    for case in cases:
        for draft in range(1, args.drafts + 1):
            folder = args.output / f"{case['id']}-{draft:02d}"
            folder.mkdir(exist_ok=True)
            record_path = folder / "result.json"
            if record_path.exists():
                old = json.loads(record_path.read_text())
                if not old.get("error"):
                    results.append(old)
                    print(f"SKIP {folder.name}: completed", flush=True)
                    continue
            record = {"case": case["id"], "draft": draft, "model": args.model,
                      "judge_model": args.judge_model or None,
                      "same_model_judge": bool(args.judge_model == args.model)}
            try:
                generation_path = folder / "generation.json"
                if generation_path.exists():
                    generated = json.loads(generation_path.read_text())
                else:
                    prompt = writing_prompt(case, args.language)
                    (folder / "prompt.txt").write_text(prompt, encoding="utf-8")
                    generated = complete(args.model, prompt, args)
                    write_json(generation_path, generated)
                script = generated["text"]
                (folder / "script.md").write_text(script, encoding="utf-8")
                record.update(checks=script_checks(script, case["target_words"]),
                              usage=generated["usage"], seconds=generated["seconds"])
                if args.judge_model:
                    prompt = judging_prompt(case, script, args.language)
                    (folder / "judge-prompt.txt").write_text(prompt, encoding="utf-8")
                    judged = complete(args.judge_model, prompt, args, judge=True)
                    write_json(folder / "judge-response.json", judged)
                    record["judgment"] = parse_judgment(judged["text"], script)
                    record["judge_usage"] = judged["usage"]
                score = record.get("judgment", {}).get("youtube_best_practices_local")
                print(f"OK {folder.name}: {record['checks']['spoken_words_approx']} words; "
                      f"score={score if score is not None else 'not judged'}", flush=True)
            except (RuntimeError, ValueError, OSError, KeyError, IndexError, TypeError) as error:
                record["error"] = str(error)
                print(f"FAILED {folder.name}: {error}", flush=True)
            write_json(record_path, record)
            results.append(record)
            write_json(args.output / "summary.json", summarize(results))
            if record.get("error"):
                return 1  # No retry storm, account rotation or rate-limit probing.
            time.sleep(args.delay)
    write_json(args.output / "summary.json", summarize(results))
    return 0
