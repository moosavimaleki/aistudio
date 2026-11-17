import copy
import json
import os
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

from examples.youtube_benchmark import load_cases, parse_args
from examples.youtube_eval.prompts import RUBRIC, judging_prompt
from examples.youtube_eval.runner import run, summarize
from examples.youtube_eval.scoring import parse_judgment, script_checks
from examples.youtube_eval.transport import complete


CASE_PATH = Path(__file__).resolve().parents[1] / "examples/youtube_eval/cases.json"
SCRIPT = "Try this cut. Subscribe for practical films. What will you film next?"


def judgment():
    return {"criteria": {name: {"evidence": ["Try this cut."], "score": 3,
                               "reason": "A synthetic fixture, not a real evaluation."}
                         for name in RUBRIC}}


def settings(output):
    return SimpleNamespace(
        output=Path(output), case=[], cases=CASE_PATH, delay=0, timeout=30,
        model="gemini-4-argon", judge_model="", base_url="http://localhost:3346",
        browser_id="", thinking=1, judge_thinking=2,
        max_output_tokens=8192, language="English", drafts=1,
    )


class ScoringTest(unittest.TestCase):
    def test_ignores_visual_directions_in_word_count(self):
        result = script_checks("One two. [SHOW: three more words] Three four.", 4)
        self.assertEqual(result["spoken_words_approx"], 4)
        self.assertEqual(result["visual_cues"], 1)
        self.assertEqual(result["length_ratio"], 1)

    def test_accepts_exact_evidence_and_scores(self):
        result = parse_judgment(json.dumps(judgment()), SCRIPT)
        self.assertEqual(result["youtube_best_practices_local"], 75)

    def test_missing_criteria_invalid_scores_and_fabricated_quotes_are_rejected(self):
        for key, value in [("score", True), ("score", 5), ("score", 1.5),
                           ("evidence", ["invented quote"]), ("evidence", []),
                           ("evidence", [""]), ("reason", "")]:
            with self.subTest(key=key, value=value):
                data = judgment()
                data["criteria"]["open_loop"][key] = value
                with self.assertRaises(ValueError):
                    parse_judgment(json.dumps(data), SCRIPT)
        with self.assertRaises(ValueError):
            parse_judgment('{"criteria":{}}', SCRIPT)

    def test_absent_feature_may_score_zero_without_evidence(self):
        data = judgment()
        data["criteria"]["open_loop"].update(score=0, evidence=[])
        parsed = parse_judgment("```json\n" + json.dumps(data) + "\n```", SCRIPT)
        self.assertEqual(parsed["youtube_best_practices_local"], 60)

    def test_no_judgments_means_no_invented_score(self):
        self.assertIsNone(summarize([{"case": "axis"}])["youtube_best_practices_local_mean"])


class CasesTest(unittest.TestCase):
    def test_selects_real_cases_and_rejects_unknown_id(self):
        self.assertEqual(len(load_cases(CASE_PATH, [])), 3)
        self.assertEqual(load_cases(CASE_PATH, ["axis"])[0]["id"], "axis")
        with self.assertRaises(ValueError):
            load_cases(CASE_PATH, ["missing"])

    def test_rejects_path_traversal_and_invalid_schema(self):
        original = load_cases(CASE_PATH, ["axis"])
        for key, value in [("id", "../private"), ("target_words", 0),
                           ("research", "not a list"), ("brief", "")]:
            with self.subTest(key=key), tempfile.TemporaryDirectory() as folder:
                data = copy.deepcopy(original)
                data[0][key] = value
                path = Path(folder) / "cases.json"
                path.write_text(json.dumps(data))
                with self.assertRaises(ValueError):
                    load_cases(path, [])

    def test_cli_defaults_and_negative_input(self):
        with patch("sys.argv", ["benchmark", "--output", "/tmp/synthetic-eval"]):
            self.assertEqual(parse_args().thinking, 1)
        with patch("sys.argv", ["benchmark", "--output", "/tmp/synthetic-eval", "--drafts", "0"]):
            with self.assertRaises(SystemExit):
                parse_args()


class TransportTest(unittest.TestCase):
    @patch("examples.youtube_eval.transport.vertex_client.generate_content")
    def test_vertex_payload_visible_text_usage_and_truncation(self, generate):
        candidate = {"content": {"parts": [{"text": "hidden", "thought": True},
                                           {"text": SCRIPT}]}, "finishReason": "STOP"}
        generate.return_value = {"candidates": [candidate], "usageMetadata": {"totalTokenCount": 100}}
        result = complete("gemini-4-argon", "write", settings("/tmp/synthetic-eval"))
        self.assertEqual(result["text"], SCRIPT)
        self.assertEqual(result["usage"]["totalTokenCount"], 100)
        payload = generate.call_args.args[1]
        self.assertEqual(payload["generationConfig"]["thinkingConfig"]["levelEnum"], 1)
        self.assertNotIn("tools", payload)
        complete("gemini-4-argon", "judge", settings("/tmp/synthetic-eval"), judge=True)
        judge_config = generate.call_args.args[1]["generationConfig"]
        self.assertEqual(judge_config["thinkingConfig"]["levelEnum"], 2)
        for finish in ["MAX_TOKENS", 2]:
            candidate["finishReason"] = finish
            with self.assertRaisesRegex(ValueError, "truncated"):
                complete("gemini-4-argon", "write", settings("/tmp/synthetic-eval"))

    @patch("examples.youtube_eval.transport.urlopen")
    def test_chatgpt_uses_openai_endpoint_and_optional_profile(self, urlopen):
        from io import BytesIO
        urlopen.return_value = BytesIO(json.dumps({"choices": [{"message": {"content": SCRIPT},
                                                               "finish_reason": "stop"}]}).encode())
        args = settings("/tmp/synthetic-eval")
        args.browser_id = "synthetic-profile"
        with patch.dict(os.environ, {"AISTUDIO_GENCONTENT_URL": args.base_url}):
            result = complete("chatgpt/test", "write", args)
        request = urlopen.call_args.args[0]
        self.assertTrue(request.full_url.endswith("/v1/chat/completions"))
        self.assertEqual(json.loads(request.data)["browser_id"], "synthetic-profile")
        self.assertEqual(result["text"], SCRIPT)


class RunnerTest(unittest.TestCase):
    @patch("examples.youtube_eval.runner.complete")
    def test_resume_skips_successes_and_rejects_changed_settings(self, complete_mock):
        complete_mock.return_value = {"text": SCRIPT, "usage": {}, "seconds": 1}
        with tempfile.TemporaryDirectory() as folder:
            args = settings(folder)
            cases = load_cases(CASE_PATH, ["axis"])
            self.assertEqual(run(args, cases), 0)
            self.assertEqual(run(args, cases), 0)
            self.assertEqual(complete_mock.call_count, 1)
            args.thinking = 3
            with self.assertRaisesRegex(ValueError, "different settings"):
                run(args, cases)

    @patch("examples.youtube_eval.runner.complete")
    def test_judge_failure_resumes_without_regenerating_draft(self, complete_mock):
        generated = {"text": SCRIPT, "usage": {}, "seconds": 1}
        judged = {"text": json.dumps(judgment()), "usage": {}, "seconds": 1}
        complete_mock.side_effect = [generated, RuntimeError("HTTP 429"), judged]
        with tempfile.TemporaryDirectory() as folder:
            args = settings(folder)
            args.judge_model = "chatgpt/synthetic"
            cases = load_cases(CASE_PATH, ["axis"])
            self.assertEqual(run(args, cases), 1)
            self.assertEqual(complete_mock.call_count, 2)  # Stops at first error.
            self.assertEqual(run(args, cases), 0)
            self.assertEqual(complete_mock.call_count, 3)
            summary = json.loads((Path(folder) / "summary.json").read_text())
            self.assertEqual(summary["youtube_best_practices_local_mean"], 75)
            self.assertNotIn(args.model, judging_prompt(cases[0], SCRIPT, "English"))


if __name__ == "__main__":
    unittest.main()
