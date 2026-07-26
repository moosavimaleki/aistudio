from __future__ import annotations

import unittest

from aistudio_client.stream import model_parts_from_chunk, visible_text_from_chunk


class StreamTests(unittest.TestCase):
    def test_extracts_only_model_text_from_positional_frames(self) -> None:
        frame = []
        frame.append([])
        frame[0].append([])
        frame[0][0].append([[[None, "hello"]], "model"])
        chunk = [[frame]]
        self.assertEqual(visible_text_from_chunk(chunk), "hello")

    def test_decodes_function_call_and_executable_code_parts(self) -> None:
        function_call = [None] * 11
        function_call[10] = [
            "lookup",
            [[["id", [None, 7]]]],
            "call-7",
        ]
        executable = [None] * 8
        executable[7] = [1, "print(7)"]
        frame = []
        frame.append([])
        frame[0].append([])
        frame[0][0].append([[function_call, executable], "model"])
        chunk = [[frame]]

        parts = model_parts_from_chunk(chunk)

        self.assertEqual(parts[0]["functionCall"]["args"], {"id": 7})
        self.assertEqual(parts[0]["functionCall"]["id"], "call-7")
        self.assertEqual(parts[1]["executableCode"]["language"], "PYTHON")
