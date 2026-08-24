from __future__ import annotations

import stat
import tempfile
import unittest
from http.cookiejar import Cookie, CookieJar
from pathlib import Path
from unittest.mock import patch
from urllib.error import HTTPError

from scripts.import_chatgpt_cookies import remove_stale_exports
from scripts.chatgpt_cookie_import import ACCOUNT_CHECK_URL, ACCOUNT_URL, discover_profiles, validate_session, write_netscape


def cookie(name: str, value: str) -> Cookie:
    return Cookie(
        version=0,
        name=name,
        value=value,
        port=None,
        port_specified=False,
        domain=".chatgpt.com",
        domain_specified=True,
        domain_initial_dot=True,
        path="/",
        path_specified=True,
        secure=True,
        expires=None,
        discard=True,
        comment=None,
        comment_url=None,
        rest={"HttpOnly": None},
        rfc2109=False,
    )


class CookieImportTests(unittest.TestCase):
    def test_discovers_cookie_databases(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            database = root / "Profile 2" / "Network" / "Cookies"
            database.parent.mkdir(parents=True)
            database.touch()

            profiles = discover_profiles([root])

            self.assertEqual([profile.cookie_db for profile in profiles], [database])

    def test_writes_private_netscape_file(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            destination = Path(directory) / "chatgpt-01.txt"
            cookies = CookieJar()
            cookies.set_cookie(cookie("session", "secret-value"))

            write_netscape(destination, cookies, replace=False)

            text = destination.read_text(encoding="utf-8")
            self.assertIn("#HttpOnly_.chatgpt.com\tTRUE\t/\tTRUE\t0\tsession\tsecret-value", text)
            self.assertEqual(stat.S_IMODE(destination.stat().st_mode), 0o640)
            with self.assertRaises(FileExistsError):
                write_netscape(destination, cookies, replace=False)

    def test_validation_accepts_authenticated_account_response(self) -> None:
        opener = FakeOpener(
            b'{"accessToken":"token"}',
            b'{"accounts":{"account":{}}, "account_ordering":["account"]}',
            b'{"id":"user", "email":"user@example.test", "client_id":"client",'
            b'"orgs":{"data":[{"id":"org", "personal":true}]}}'
        )

        with patch("scripts.chatgpt_cookie_import.build_opener", return_value=opener):
            active, reason = validate_session(CookieJar(), proxy="", timeout=3)

        self.assertTrue(active)
        self.assertEqual(reason, "active authenticated account")
        self.assertEqual(
            [request.full_url for request in opener.requests],
            ["https://chatgpt.com/api/auth/session", ACCOUNT_CHECK_URL, ACCOUNT_URL],
        )
        self.assertEqual(opener.requests[2].get_header("Chatgpt-account-id"), "account")
        self.assertEqual(opener.requests[2].get_header("Authorization"), "Bearer token")

    def test_validation_rejects_half_signed_in_account_response(self) -> None:
        opener = FakeOpener(
            b'{"accessToken":"token"}',
            b'{"accounts":{"account":{}}, "account_ordering":["account"]}',
            b'{"id":"guest", "email":"guest@example.test", "object":"user"}',
        )

        with patch("scripts.chatgpt_cookie_import.build_opener", return_value=opener):
            active, reason = validate_session(CookieJar(), proxy="", timeout=3)

        self.assertFalse(active)
        self.assertEqual(reason, "account response is anonymous or incomplete")

    def test_validation_rejects_unauthorized_backend_response(self) -> None:
        opener = FakeOpener(
            b'{"accessToken":"token"}',
            b'{"accounts":{"account":{}}, "account_ordering":["account"]}',
            HTTPError(ACCOUNT_URL, 401, "Unauthorized", {}, None),
        )

        with patch("scripts.chatgpt_cookie_import.build_opener", return_value=opener):
            active, reason = validate_session(CookieJar(), proxy="", timeout=3)

        self.assertFalse(active)
        self.assertEqual(reason, "account endpoint returned HTTP 401")

    def test_validation_rejects_session_without_access_token(self) -> None:
        opener = FakeOpener(b'{"user":{"id":"guest"}}')

        with patch("scripts.chatgpt_cookie_import.build_opener", return_value=opener):
            active, reason = validate_session(CookieJar(), proxy="", timeout=3)

        self.assertFalse(active)
        self.assertEqual(reason, "session access token is missing")

    def test_replace_prunes_only_obsolete_numbered_exports(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory)
            (output / "chatgpt-01.txt").touch()
            (output / "chatgpt-02.txt").touch()
            (output / "chatgpt.txt").touch()
            (output / "notes.txt").touch()

            remove_stale_exports(output, {"chatgpt-01.txt"})

            self.assertTrue((output / "chatgpt-01.txt").exists())
            self.assertFalse((output / "chatgpt-02.txt").exists())
            self.assertTrue((output / "chatgpt.txt").exists())
            self.assertTrue((output / "notes.txt").exists())


class FakeResponse:
    def __init__(self, body: bytes) -> None:
        self.body = body

    def __enter__(self) -> "FakeResponse":
        return self

    def __exit__(self, *_args: object) -> None:
        return None

    def read(self) -> bytes:
        return self.body


class FakeOpener:
    def __init__(self, *results: bytes | HTTPError) -> None:
        self.results = list(results)
        self.requests = []

    def open(self, request, timeout: float):
        self.requests.append(request)
        result = self.results.pop(0)
        if isinstance(result, HTTPError):
            raise result
        return FakeResponse(result)


if __name__ == "__main__":
    unittest.main()
