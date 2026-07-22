from unittest import IsolatedAsyncioTestCase, TestCase
from unittest.mock import AsyncMock

from browser_interface.browser.cookies import apply_and_verify
from browser_interface.browser.cookies import google_cookie_records
from browser_interface.browser.cookies import parse_google_cookies, session_fingerprint
from browser_interface.browser.navigation import document_url


class CookieTests(TestCase):
    def test_account_index_is_encoded_in_document_url(self):
        self.assertEqual(
            document_url("0"),
            "https://aistudio.google.com/prompts/new_chat",
        )
        self.assertEqual(
            document_url("1"),
            "https://aistudio.google.com/u/1/prompts/new_chat",
        )

    def test_header_becomes_secure_google_cookies(self):
        self.assertEqual(
            parse_google_cookies("SID=one; SAPISID=two"),
            [
                _cookie("SID", "one"),
                _cookie("SAPISID", "two"),
            ],
        )

    def test_host_cookies_are_not_reconstructed(self):
        self.assertEqual(parse_google_cookies("__Host-test=value"), [])

    def test_only_google_cookies_are_returned(self):
        self.assertEqual(
            google_cookie_records(
                [
                    {"domain": ".google.com", "name": "SID", "value": "one"},
                    {"domain": ".example.test", "name": "x", "value": "two"},
                ]
            ),
            [{"name": "SID", "value": "one"}],
        )

    def test_fingerprint_changes_with_account_and_authentication(self):
        baseline = session_fingerprint("SID=one; SAPISID=two", "0")
        self.assertNotEqual(
            baseline,
            session_fingerprint("SID=one; SAPISID=two", "1"),
        )
        self.assertNotEqual(
            baseline,
            session_fingerprint("SID=three; SAPISID=two", "0"),
        )


class CookieApplicationTests(IsolatedAsyncioTestCase):
    async def test_rotated_non_auth_cookie_does_not_invalidate_session(self):
        context = AsyncMock()
        context.cookies.return_value = [
            _cookie("SID", "sid"),
            _cookie("_ga_test", "rotated"),
        ]

        await apply_and_verify(
            context,
            [_cookie("SID", "sid"), _cookie("_ga_test", "original")],
        )


def _cookie(name: str, value: str) -> dict:
    return {
        "name": name,
        "value": value,
        "domain": ".google.com",
        "path": "/",
        "secure": True,
    }
