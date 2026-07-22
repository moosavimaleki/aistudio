from __future__ import annotations

import unittest

from aistudio_client.auth import AuthContext, generate_authorization_header, normalize_origin


class AuthTests(unittest.TestCase):
    def test_normalizes_origin_and_builds_all_secure_proofs(self) -> None:
        context = AuthContext(
            "https://aistudio.google.com/prompts/new_chat?x=1",
            "SAPISID=primary; __Secure-1PAPISID=first; __Secure-3PAPISID=third",
            clock=lambda: 1_720_000_000,
        )
        header = generate_authorization_header(context)
        self.assertEqual(normalize_origin("https://aistudio.google.com/x"), "https://aistudio.google.com")
        self.assertEqual(len(header.split(" SAPISID")), 3)
        self.assertIn("SAPISIDHASH 1720000000_", header)
        self.assertIn("SAPISID1PHASH 1720000000_", header)
        self.assertIn("SAPISID3PHASH 1720000000_", header)

    def test_rejects_non_http_origin(self) -> None:
        with self.assertRaises(ValueError):
            normalize_origin("chrome-extension://abc")
