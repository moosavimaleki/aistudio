import os
from unittest import IsolatedAsyncioTestCase
from unittest.mock import AsyncMock, Mock, patch

from browser_interface.services.tokens import TokenService


class TokenServiceTests(IsolatedAsyncioTestCase):
    async def test_same_browser_activation_runs_once_per_session(self):
        browser = Mock()
        browser.fingerprint = "session-one"
        browser.prepare = AsyncMock(return_value=_session())
        browser.snapshot = AsyncMock(return_value=_session())
        browser.probe = AsyncMock(return_value={"status": 200, "body": "ok"})

        browsers = Mock()
        browsers.resolve.return_value = "default"
        browsers.session.return_value = browser
        browsers.assert_identity = Mock()
        broker = Mock()
        broker.request = AsyncMock(side_effect=[
            {"token": "candidate", "candidateTokens": ["candidate"]},
            {"token": "first-request"},
            {"token": "second-request", "candidateTokens": ["second-request"]},
        ])
        service = TokenService(broker, browsers)

        with (
            patch.dict(os.environ, {"TOKEN_FACTORY_SAME_BROWSER_PROBE": "1"}),
            patch(
                "browser_interface.services.tokens.validate_token_request",
                return_value=(_headers(), "0"),
            ),
        ):
            first = await service.create(_body())
            second = await service.create(_body())

        self.assertEqual(first["token"], "first-request")
        self.assertEqual(second["token"], "second-request")
        browser.probe.assert_awaited_once()


def _body() -> dict:
    return {
        "attestationEnabled": True,
        "digest": "a" * 64,
        "cookies": "SAPISID=session",
        "generateRequest": {"payload": [], "url": "https://example.test"},
    }


def _headers() -> dict[str, str]:
    return {
        "user-agent": "Chrome/136",
        "x-client-data": "client-data",
        "x-goog-api-key": "api-key",
        "x-goog-authuser": "0",
    }


def _session() -> dict:
    return {
        "runtimeConfig": {"apiKey": "api-key", "authUser": "0"},
        "transportProfile": {
            "User-Agent": "Chrome/136",
            "x-client-data": "client-data",
        },
        "cookieRecords": [{"name": "SAPISID", "value": "session"}],
    }
