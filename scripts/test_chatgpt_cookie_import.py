from __future__ import annotations

import stat
import tempfile
import unittest
from http.cookiejar import Cookie, CookieJar
from pathlib import Path

from scripts.chatgpt_cookie_import import discover_profiles, write_netscape


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


if __name__ == "__main__":
    unittest.main()
