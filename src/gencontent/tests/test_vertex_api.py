import unittest
import json
from types import SimpleNamespace

from pydantic import ValidationError

from aistudio_client.models import GenerateResult
from gencontent.api.adapters import vertex_input, vertex_response
from gencontent.api.models import VertexGenerateContentBody
from gencontent.api.sse import vertex_sse


class VertexApiTests(unittest.TestCase):
    def test_inline_data_is_accepted_as_vertex_part(self):
        body = VertexGenerateContentBody.model_validate({
            "contents": [{
                "role": "user",
                "parts": [{"inlineData": {
                    "data": "dm9pY2U=",
                    "mimeType": "audio/ogg",
                }}],
            }],
            "generationConfig": {"thinkingConfig": {"levelEnum": 4}},
        })

        request = vertex_input("gemini-test", body)

        self.assertEqual(
            request.contents[0]["parts"][0]["inlineData"]["data"],
            "dm9pY2U=",
        )

    def test_request_maps_vertex_fields_to_generate_input(self):
        body = VertexGenerateContentBody.model_validate({
            "contents": [{"role": "user", "parts": [{"text": "سلام"}]}],
            "systemInstruction": {"parts": [{"text": "کوتاه جواب بده"}]},
            "generationConfig": {
                "temperature": 0.25,
                "topP": 0.8,
                "topK": 20,
                "maxOutputTokens": 100,
                "thinkingConfig": {"thinkingBudget": 64},
            },
        })

        generated = vertex_input("gemini-test", body)

        self.assertEqual(generated.model, "models/gemini-test")
        self.assertEqual(generated.contents[0]["parts"][0]["text"], "سلام")
        self.assertEqual(generated.generation_config["temperature"], 0.25)
        self.assertEqual(generated.generation_config["maxOutputTokens"], 100)
        self.assertEqual(generated.generation_config["thinkingConfig"]["thinkingBudget"], 64)
        self.assertEqual(
            generated.system_instruction["parts"][0]["text"],
            "کوتاه جواب بده",
        )

    def test_candidate_count_is_rejected_at_http_contract(self):
        with self.assertRaises(ValidationError):
            VertexGenerateContentBody.model_validate({
                "contents": [{"role": "user", "parts": [{"text": "سلام"}]}],
                "generationConfig": {
                    "candidateCount": 2,
                    "thinkingConfig": {"levelEnum": 4},
                },
            })

    def test_thinking_mode_is_required(self):
        with self.assertRaises(ValidationError):
            VertexGenerateContentBody.model_validate({
                "contents": [{"role": "user", "parts": [{"text": "سلام"}]}],
                "generationConfig": {"temperature": 0},
            })

    def test_response_uses_vertex_candidate_shape(self):
        outcome = SimpleNamespace(
            tab_id="tab-1",
            browser_id="browser2",
            auth_user="0",
            generate_count=3,
            result=GenerateResult(final_text="پاسخ", chunks=[["frame"]]),
        )

        response = vertex_response("gemini-test", outcome)

        self.assertEqual(response["candidates"][0]["content"]["role"], "model")
        self.assertEqual(response["candidates"][0]["content"]["parts"], [{"text": "پاسخ"}])
        self.assertEqual(response["labMetadata"]["browserId"], "browser2")

    def test_sse_frame_is_accepted_by_google_genai_transport(self):
        frame = next(iter(vertex_sse({"candidates": []})))

        self.assertTrue(frame.startswith("data: "))
        self.assertTrue(frame.endswith("\n\n"))
        self.assertEqual(json.loads(frame[6:]), {"candidates": []})


if __name__ == "__main__":
    unittest.main()
