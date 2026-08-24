from __future__ import annotations

import stat
import tempfile
import unittest
from http.cookiejar import Cookie, CookieJar
from pathlib import Path
from unittest.mock import patch
from urllib.error import HTTPError

from scripts.chatgpt_cookie_import import CONVERSATIONS_URL, discover_profiles, validate_session, write_netscape


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

    def test_validation_accepts_authenticated_conversations_response(self) -> None:
        opener = FakeOpener(b'{"items": [], "total": 0, "limit": 1, "offset": 0}')

        with patch("scripts.chatgpt_cookie_import.build_opener", return_value=opener):
            active, reason = validate_session(CookieJar(), proxy="", timeout=3)

        self.assertTrue(active)
        self.assertEqual(reason, "active backend session")
        self.assertEqual(opener.request.full_url, CONVERSATIONS_URL)

    def test_validation_rejects_session_like_but_not_backend_response(self) -> None:
        opener = FakeOpener(b'{"accessToken": "still-not-enough"}')

        with patch("scripts.chatgpt_cookie_import.build_opener", return_value=opener):
            active, reason = validate_session(CookieJar(), proxy="", timeout=3)

        self.assertFalse(active)
        self.assertEqual(reason, "conversations response was not an authenticated page")

    def test_validation_rejects_unauthorized_backend_response(self) -> None:
        opener = FakeOpener(HTTPError(CONVERSATIONS_URL, 401, "Unauthorized", {}, None))

        with patch("scripts.chatgpt_cookie_import.build_opener", return_value=opener):
            active, reason = validate_session(CookieJar(), proxy="", timeout=3)

        self.assertFalse(active)
        self.assertEqual(reason, "conversations endpoint returned HTTP 401")


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
    def __init__(self, result: bytes | HTTPError) -> None:
        self.result = result
        self.request = None

    def open(self, request, timeout: float):
        self.request = request
        if isinstance(self.result, HTTPError):
            raise self.result
        return FakeResponse(self.result)


if __name__ == "__main__":
    unittest.main()
