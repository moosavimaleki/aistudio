from __future__ import annotations

import base64
import json
import unittest

from aistudio_client.media.multipart import build_multipart


class MultipartTests(unittest.TestCase):
    def test_builds_drive_related_upload(self):
        boundary, body = build_multipart("voice.ogg", "folder-1", "audio/ogg", b"voice")
        text = body.decode("utf-8")

        self.assertIn(f"--{boundary}\r\n", text)
        self.assertIn(json.dumps({"name": "voice.ogg", "parents": ["folder-1"]}, separators=(",", ":")), text)
        self.assertIn("Content-Type: audio/ogg", text)
        self.assertIn(base64.b64encode(b"voice").decode(), text)
        self.assertTrue(text.endswith(f"--{boundary}--\r\n"))
