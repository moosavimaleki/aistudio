from __future__ import annotations

import unittest

from aistudio_client.stream import visible_text_from_chunk


class StreamTests(unittest.TestCase):
    def test_extracts_only_model_text_from_positional_frames(self) -> None:
        frame = []
        frame.append([])
        frame[0].append([])
        frame[0][0].append([[[None, "hello"]], "model"])
        chunk = [[frame]]
        self.assertEqual(visible_text_from_chunk(chunk), "hello")
