from __future__ import annotations

import json
import unittest

from aistudio_client.auth import AuthContext
from aistudio_client.models import RuntimeConfig
from aistudio_client.token_factory import StagingTokenFactory


class FakeResponse:
    ok = True
    status_code = 200
    text = '{"token":"fixture-token"}'

    @staticmethod
    def json():
        return {"token": "fixture-token", "cookieRecords": [{"name": "SIDCC", "value": "rotated"}]}


class FakeHttp:
    def __init__(self) -> None:
        self.kwargs = None

    def request(self, *_args, **kwargs):
        self.kwargs = kwargs
        return FakeResponse()


class TokenFactoryTests(unittest.TestCase):
    def test_encodes_persian_context_as_utf8_bytes(self) -> None:
        http = FakeHttp()
        auth = AuthContext("https://aistudio.google.com/", "SAPISID=a; __Secure-1PAPISID=a; __Secure-3PAPISID=a", clock=lambda: 1)
        factory = StagingTokenFactory(
            http,
            "http://localhost:3344/get-token",
            "waa",
            auth,
            RuntimeConfig("key", "visit", "0"),
            browser_id="browser2",
        )
        snapshot = factory.snapshot("a" * 64, {"method": "POST", "payload": ["سلام"], "headers": {}})
        self.assertEqual(snapshot.token, "fixture-token")
        self.assertIsInstance(http.kwargs["data"], bytes)
        self.assertEqual(json.loads(http.kwargs["data"].decode())["generateRequest"]["payload"], ["سلام"])
        self.assertEqual(json.loads(http.kwargs["data"].decode())["browserId"], "browser2")
