from __future__ import annotations

import json
import unittest

from aistudio_client.bootstrap import fetch_runtime_config
from aistudio_client.cookies import CookieJar


class FakeResponse:
    ok = True
    status_code = 200
    text = ""

    def json(self):
        return {
            "runtimeConfig": {
                "apiKey": "container-key",
                "visitId": "container-visit",
                "authUser": "1",
                "attestationEnabled": True,
            },
            "transportProfile": {
                "User-Agent": "Container Chrome/136",
                "x-client-data": "container-client-data",
            },
            "cookieRecords": [{"name": "SIDCC", "value": "rotated"}],
        }


class FakeHttp:
    def __init__(self):
        self.calls = []

    def request(self, method, url, **kwargs):
        self.calls.append((method, url, kwargs))
        return FakeResponse()


class BootstrapTests(unittest.TestCase):
    def test_configured_factory_is_the_authoritative_browser_bootstrap(self):
        http = FakeHttp()
        cookies = CookieJar("SAPISID=session")

        runtime, profile = fetch_runtime_config(
            http,
            cookies,
            token_factory_url="http://localhost:3345/get-token",
            auth_user="1",
            browser_id="browser2",
        )

        self.assertEqual(len(http.calls), 1)
        method, url, kwargs = http.calls[0]
        self.assertEqual((method, url), ("POST", "http://localhost:3345/bootstrap"))
        request_body = json.loads(kwargs["data"])
        self.assertEqual(request_body["authUser"], "1")
        self.assertEqual(request_body["browserId"], "browser2")
        self.assertEqual(runtime.api_key, "container-key")
        self.assertEqual(runtime.visit_id, "container-visit")
        self.assertEqual(profile["User-Agent"], "Container Chrome/136")
        self.assertIn("SIDCC=rotated", cookies.header)


if __name__ == "__main__":
    unittest.main()
