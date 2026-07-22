from pathlib import Path
from tempfile import TemporaryDirectory
from unittest import TestCase

from browser_interface.cookie_source import read_cookie_file
from browser_interface.cookie_store import persist_cookie_file


class CookieStoreTests(TestCase):
    def test_live_chrome_cookies_replace_profile_file(self):
        with TemporaryDirectory() as directory:
            path = Path(directory) / "account.txt"
            path.write_text("old", encoding="utf-8")

            count = persist_cookie_file(
                path,
                [
                    {
                        "name": "SAPISID",
                        "value": "rotated",
                        "domain": ".google.com",
                        "path": "/",
                        "secure": True,
                        "httpOnly": True,
                        "expires": 1_900_000_000.5,
                    },
                    {
                        "name": "ignored",
                        "value": "outside",
                        "domain": ".example.test",
                    },
                ],
            )

            self.assertEqual(count, 1)
            self.assertEqual(read_cookie_file(str(path)), "SAPISID=rotated")
            self.assertIn("#HttpOnly_.google.com", path.read_text(encoding="utf-8"))
            self.assertEqual(path.stat().st_gid, path.parent.stat().st_gid)

    def test_host_cookie_is_not_persisted(self):
        with TemporaryDirectory() as directory:
            path = Path(directory) / "account.txt"
            count = persist_cookie_file(
                path,
                [{"name": "__Host-test", "value": "x", "domain": "google.com"}],
            )
            self.assertEqual(count, 0)
