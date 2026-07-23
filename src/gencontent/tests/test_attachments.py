from __future__ import annotations

import base64
import unittest

from gencontent.attachments import resolve_inline_data


class FakeTab:
    def upload_bytes(self, content, *, mime_type, name=None):
        self.call = (content, mime_type, name)
        return "drive-file-1"


class AttachmentTests(unittest.TestCase):
    def test_inline_data_is_uploaded_without_mutating_request(self):
        contents = [{"role": "user", "parts": [{"inlineData": {
            "data": base64.b64encode(b"voice").decode(),
            "mimeType": "audio/ogg",
            "displayName": "sample.ogg",
        }}]}]
        tab = FakeTab()

        resolved = resolve_inline_data(contents, tab)

        self.assertEqual(tab.call, (b"voice", "audio/ogg", "sample.ogg"))
        self.assertEqual(resolved[0]["parts"][0]["fileData"], {
            "mimeType": "audio/ogg",
            "fileId": "drive-file-1",
        })
        self.assertIn("inlineData", contents[0]["parts"][0])

    def test_invalid_base64_is_rejected(self):
        contents = [{"role": "user", "parts": [{"inlineData": {
            "data": "not-base64!", "mimeType": "audio/ogg",
        }}]}]
        with self.assertRaisesRegex(ValueError, "valid base64"):
            resolve_inline_data(contents, FakeTab())

    def test_google_genai_urlsafe_base64_is_accepted(self):
        contents = [{"role": "user", "parts": [{"inlineData": {
            "data": "-_8=", "mimeType": "application/octet-stream",
        }}]}]
        tab = FakeTab()

        resolved = resolve_inline_data(contents, tab)

        self.assertEqual(tab.call, (b"\xfb\xff", "application/octet-stream", None))
        self.assertEqual(
            resolved[0]["parts"][0]["fileData"]["fileId"],
            "drive-file-1",
        )
