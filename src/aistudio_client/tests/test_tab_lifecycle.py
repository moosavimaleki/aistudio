from __future__ import annotations

import unittest
from unittest.mock import patch

from aistudio_client.auth import AuthContext
from aistudio_client.client import AIStudioClient
from aistudio_client.cookies import CookieJar
from aistudio_client.errors import ClientError
from aistudio_client.models import GenerateInput, GenerateResult, RuntimeConfig
from aistudio_client.tab import AIStudioTab, TabState, invalidates_tab
from aistudio_client.tab_snapshot import dump_tab, restore_tab
from aistudio_client.token_factory import TokenSnapshot


class FakeSettings:
    model = "models/test"
    proxy_url = None
    origin_url = "https://aistudio.google.com/prompts/new_chat"
    cookie_header = "SAPISID=s; __Secure-1PAPISID=one"
    auth_user = "1"
    browser_id = "default"
    token_factory_url = "http://browser-interface:3345/get-token"
    waa_api_key = "waa-test-key"
    values = {}


class FakeTab:
    def __init__(self, outcomes, *, initialize_error=None):
        self.outcomes = list(outcomes)
        self.initialize_error = initialize_error
        self.state = TabState.NEW
        self.initialize_count = 0
        self.generate_count = 0
        self.closed = False
        self.id = f"tab-{id(self)}"
        self.http = None
        self.runtime = None
        self.transport_profile = None
        self.logging_context_extension = None
        self.oauth_access_token = None

    def initialize(self):
        self.initialize_count += 1
        if self.initialize_error:
            raise self.initialize_error
        self.state = TabState.READY
        return self

    def generate(self, _input, *, on_chunk=None):
        self.generate_count += 1
        outcome = self.outcomes.pop(0)
        if isinstance(outcome, BaseException):
            raise outcome
        return outcome

    def close(self):
        self.closed = True
        self.state = TabState.CLOSED


class FakeHeaders(dict):
    pass


class FakeGenerateResponse:
    def __init__(self, status_code, body, *, set_cookie=None):
        self.status_code = status_code
        self.ok = 200 <= status_code < 300
        self.text = body
        self.headers = FakeHeaders()
        if set_cookie:
            self.headers["set-cookie"] = set_cookie
        self.raw = None
        self.closed = False

    def iter_lines(self, decode_unicode=True):
        return [self.text]

    def close(self):
        self.closed = True


class FakeGenerateHttp:
    def __init__(self, responses):
        self.responses = list(responses)
        self.payload_tokens = []

    def request(self, _method, _url, **kwargs):
        self.payload_tokens.append(kwargs["json"][4])
        return self.responses.pop(0)


class FakeTokenFactory:
    def __init__(self):
        self.snapshot_count = 0
        self.auth = None
        self.runtime = None
        self.session_headers = []

    def snapshot(self, _digest, _request):
        self.snapshot_count += 1
        return TokenSnapshot(f"token-{self.snapshot_count}")

    def update_session(self, auth, runtime):
        self.auth = auth
        self.runtime = runtime
        self.session_headers.append(auth.cookie_header)


class TabLifecycleTests(unittest.TestCase):
    def test_container_fingerprint_mismatch_invalidates_persisted_tab(self):
        mismatch = ClientError(
            "Token factory failed with HTTP 500",
            phase="ATTESTATION",
            status=500,
            response_body='{"error":"GenerateContent x-client-data differs from container Chrome"}',
        )
        unrelated = ClientError("upstream unavailable", phase="ATTESTATION", status=500)

        self.assertTrue(invalidates_tab(mismatch))
        self.assertFalse(invalidates_tab(unrelated))

    def test_ready_tab_round_trips_without_bootstrap(self):
        cookies = CookieJar("SAPISID=s; __Secure-1PAPISID=one")
        tab = AIStudioTab.__new__(AIStudioTab)
        tab.id = "tab-persisted"
        tab.settings = FakeSettings()
        tab.http = None
        tab.state = TabState.READY
        tab.cookies = cookies
        tab.auth = AuthContext(FakeSettings.origin_url, cookies.header)
        tab.runtime = RuntimeConfig("api-key", "visit-id", "1")
        tab.transport_profile = {"User-Agent": "Container Chrome/136"}
        tab.logging_context_extension = "logging-context"
        tab.oauth_access_token = "oauth"
        tab.token_factory = None
        tab.generate_count = 12

        state = dump_tab(tab)
        restored = restore_tab(FakeSettings(), state, http=FakeGenerateHttp([]))

        self.assertEqual(state["browserId"], "default")
        self.assertEqual(state["authUser"], "1")
        self.assertEqual(restored.id, "tab-persisted")
        self.assertEqual(restored.state, TabState.READY)
        self.assertEqual(restored.runtime.visit_id, "visit-id")
        self.assertEqual(restored.cookies.header, cookies.header)
        self.assertEqual(restored.generate_count, 12)
        self.assertEqual(restored.token_factory.runtime, restored.runtime)

    def test_reuses_one_initialized_tab_for_many_generates(self):
        result1 = GenerateResult(final_text="one")
        result2 = GenerateResult(final_text="two")
        tabs = []

        def factory(_settings):
            tab = FakeTab([result1, result2])
            tabs.append(tab)
            return tab

        client = AIStudioClient(settings=FakeSettings(), tab_factory=factory).initialize()
        tab_id = client.active_tab.id

        self.assertIs(client.generate(GenerateInput("models/test", "1")), result1)
        self.assertIs(client.generate(GenerateInput("models/test", "2")), result2)
        self.assertEqual(len(tabs), 1)
        self.assertEqual(tabs[0].initialize_count, 1)
        self.assertEqual(tabs[0].generate_count, 2)
        self.assertEqual(client.active_tab.id, tab_id)

    def test_unauthorized_tab_is_discarded_and_retried_once(self):
        expected = GenerateResult(final_text="fresh tab")
        planned = [
            [ClientError("expired runtime", status=403)],
            [expected],
        ]
        tabs = []

        def factory(_settings):
            tab = FakeTab(planned[len(tabs)])
            tabs.append(tab)
            return tab

        client = AIStudioClient(settings=FakeSettings(), tab_factory=factory).initialize()
        result = client.generate(GenerateInput("models/test", "retry"))

        self.assertIs(result, expected)
        self.assertEqual(len(tabs), 2)
        self.assertTrue(tabs[0].closed)
        self.assertEqual(tabs[1].initialize_count, 1)
        self.assertIs(client.active_tab, tabs[1])

    def test_schema_error_does_not_replace_tab(self):
        error = ClientError("invalid argument", status=400)
        tabs = []

        def factory(_settings):
            tab = FakeTab([error])
            tabs.append(tab)
            return tab

        client = AIStudioClient(settings=FakeSettings(), tab_factory=factory).initialize()
        with self.assertRaises(ClientError):
            client.generate(GenerateInput("models/test", "bad schema"))

        self.assertEqual(len(tabs), 1)
        self.assertFalse(tabs[0].closed)

    def test_unauthorized_initialization_does_not_keep_a_broken_tab(self):
        tab = FakeTab([], initialize_error=ClientError("bad keys", status=401))
        client = AIStudioClient(settings=FakeSettings(), tab_factory=lambda _settings: tab)

        with self.assertRaises(ClientError):
            client.initialize()

        self.assertTrue(tab.closed)
        self.assertIsNone(client.active_tab)
        self.assertEqual(client.state, TabState.INVALID)

    @patch("aistudio_client.tab.time.sleep")
    def test_retry_keeps_tab_but_requests_a_fresh_token(self, _sleep):
        rate_limited = FakeGenerateResponse(429, '[[8,"quota"]]', set_cookie="SIDCC=rotated; Path=/")
        succeeded = FakeGenerateResponse(200, '{"text":"ok"}')
        http = FakeGenerateHttp([rate_limited, succeeded])
        factory = FakeTokenFactory()
        cookies = CookieJar(
            "SAPISID=s; __Secure-1PAPISID=s; __Secure-3PAPISID=s"
        )
        runtime = RuntimeConfig("key", "visit", "1")

        tab = AIStudioTab.__new__(AIStudioTab)
        tab.id = "fixture-tab"
        tab.state = TabState.READY
        tab.http = http
        tab.cookies = cookies
        tab.auth = AuthContext("https://aistudio.google.com/", cookies.header)
        tab.runtime = runtime
        tab.transport_profile = {"User-Agent": "Container Chrome/136"}
        tab.logging_context_extension = None
        tab.token_factory = factory
        tab.generate_count = 0

        result = tab.generate(GenerateInput(
            "models/test",
            "hello",
            generation_config={"thinkingConfig": {"levelEnum": 4}},
        ))

        self.assertEqual(result.final_text, "ok")
        self.assertEqual(factory.snapshot_count, 2)
        self.assertEqual(http.payload_tokens, ["token-1", "token-2"])
        self.assertTrue(rate_limited.closed)
        self.assertIn("SIDCC=rotated", factory.auth.cookie_header)
        self.assertEqual(tab.generate_count, 1)
        self.assertEqual(tab.state, TabState.READY)


if __name__ == "__main__":
    unittest.main()
