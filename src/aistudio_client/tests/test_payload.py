from __future__ import annotations

import hashlib
import unittest

from aistudio_client.makersuite import build_generate_content_payload, content_binding_digest
from aistudio_client.models import GenerateInput


THINKING = {"thinkingConfig": {"levelEnum": 4}}


class PayloadTests(unittest.TestCase):
    def test_file_id_participates_in_attestation_digest(self) -> None:
        payload = build_generate_content_payload(GenerateInput(
            model="models/test",
            contents=[{"role": "user", "parts": [
                {"fileData": {"fileId": "drive-file-1"}},
                {"text": "رونویسی کن"},
            ]}],
            generation_config=THINKING,
        ))

        expected = hashlib.sha256("drive-file-1 رونویسی کن".encode()).hexdigest()
        self.assertEqual(content_binding_digest(payload), expected)

    def test_schema_and_digest_are_stable(self) -> None:
        payload = build_generate_content_payload(GenerateInput(
            model="models/gemini-3.5-flash-lite",
            prompt="سلام",
            generation_config=THINKING,
        ))
        self.assertEqual(len(payload), 11)
        self.assertEqual(payload[1], [[[[None, "سلام"]], "user"]])
        self.assertEqual(payload[3][16], [False, None, None, 4])
        self.assertEqual(content_binding_digest(payload), "bda1fa48345336618741fd2c4bc02809eb099c49a9b02fb5056401ab6d4dc3e6")

    def test_history_precedes_latest_user_turn(self) -> None:
        payload = build_generate_content_payload(GenerateInput(
            model="models/gemini-3.5-flash-lite", prompt="next", history=[{"role": "user", "text": "before"}, {"role": "model", "text": "answer"}],
            generation_config=THINKING,
        ))
        self.assertEqual([turn[1] for turn in payload[1]], ["user", "model", "user"])

    def test_vertex_generation_config_uses_confirmed_positions(self) -> None:
        payload = build_generate_content_payload(GenerateInput(
            model="models/gemini-test",
            contents=[{"role": "user", "parts": [{"text": "سلام"}]}],
            generation_config={
                "maxOutputTokens": 123,
                "temperature": 0.2,
                "topP": 0.7,
                "topK": 12,
                "stopSequences": ["END"],
                "seed": 42,
                **THINKING,
            },
        ))

        config = payload[3]
        self.assertEqual(config[1], ["END"])
        self.assertEqual(config[3:7], [123, 0.2, 0.7, 12])
        self.assertEqual(config[18], 42)

    def test_system_instruction_and_file_reference_are_encoded(self) -> None:
        payload = build_generate_content_payload(GenerateInput(
            model="models/gemini-test",
            contents=[{
                "role": "user",
                "parts": [{"fileData": {"fileId": "drive-id"}}, {"text": "توضیح بده"}],
            }],
            system_instruction={"parts": [{"text": "دقیق باش"}]},
            generation_config=THINKING,
        ))

        self.assertEqual(payload[1][0][0][0][5], ["drive-id"])
        self.assertEqual(payload[5], [[[None, "دقیق باش"]], "user"])

    def test_response_schema_is_encoded(self) -> None:
        payload = build_generate_content_payload(GenerateInput(
            model="models/gemini-test",
            prompt="json",
            generation_config={
                "responseSchema": {
                    "type": "object",
                    "properties": {"name": {"type": "string"}},
                "required": ["name"],
                },
                **THINKING,
            },
        ))

        self.assertEqual(payload[3][7], "application/json")
        self.assertEqual(payload[3][8][0], 6)
        self.assertEqual(payload[3][8][6], [["name", [1]]])

    def test_candidate_count_is_rejected_until_mapping_is_known(self) -> None:
        with self.assertRaisesRegex(ValueError, "candidateCount"):
            build_generate_content_payload(GenerateInput(
                model="models/gemini-test",
                prompt="سلام",
                generation_config={"candidateCount": 2, **THINKING},
            ))

    def test_thinking_mode_has_no_implicit_default(self) -> None:
        with self.assertRaisesRegex(ValueError, "thinkingBudget or levelEnum"):
            build_generate_content_payload(GenerateInput(
                model="models/gemini-test",
                prompt="سلام",
            ))
