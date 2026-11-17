from __future__ import annotations

import json
import stat
import tempfile
import unittest
from http.cookiejar import Cookie, CookieJar
from pathlib import Path
from unittest.mock import patch
from urllib.error import HTTPError
from types import SimpleNamespace

from scripts.import_chatgpt_cookies import main, parse_args, remove_stale_exports, should_sync_profile
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
    def test_importer_uses_the_lab_smart_proxy_by_default(self) -> None:
        with patch.dict("os.environ", {}, clear=True), patch("sys.argv", ["import_chatgpt_cookies.py"]):
            args = parse_args()

        self.assertEqual(args.proxy, "http://127.0.0.1:10809")

    def test_discovers_cookie_databases(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            database = root / "Profile 2" / "Network" / "Cookies"
            database.parent.mkdir(parents=True)
            database.touch()
            (root / "Local State").write_text(
                json.dumps({"profile": {"info_cache": {"Profile 2": {"name": "Personal"}}}}),
                encoding="utf-8",
            )

            profiles = discover_profiles([root])

            self.assertEqual([profile.cookie_db for profile in profiles], [database])
            self.assertEqual(profiles[0].name, "Personal")

    def test_excludes_profiles_whose_display_name_starts_with_x(self) -> None:
        self.assertFalse(should_sync_profile("x private"))
        self.assertFalse(should_sync_profile("X Work"))
        self.assertTrue(should_sync_profile("Work x"))

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
            b'{"accessToken":"token","user":{"id":"user","email":"user@example.test"}}',
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
        self.assertEqual(opener.requests[1].get_header("Authorization"), "Bearer token")

    def test_validation_accepts_current_me_without_legacy_org_fields(self) -> None:
        opener = FakeOpener(
            b'{"accessToken":"token","user":{"id":"user","email":"user@example.test"}}',
            b'{"accounts":{"account":{}},"account_ordering":["account"]}',
            b'{"object":"user","id":"user","email":"user@example.test"}',
        )
        with patch("scripts.chatgpt_cookie_import.build_opener", return_value=opener):
            self.assertTrue(validate_session(CookieJar(), "", 3)[0])

    def test_validation_rejects_half_signed_in_account_response(self) -> None:
        opener = FakeOpener(
            b'{"accessToken":"token","user":{"id":"user","email":"user@example.test"}}',
            b'{"accounts":{"account":{}}, "account_ordering":["account"]}',
            b'{"id":"guest", "email":"guest@example.test", "object":"user"}',
        )

        with patch("scripts.chatgpt_cookie_import.build_opener", return_value=opener):
            active, reason = validate_session(CookieJar(), proxy="", timeout=3)

        self.assertFalse(active)
        self.assertEqual(reason, "account identity does not match the authenticated session")

    def test_zero_valid_sessions_preserves_previous_exports(self) -> None:
        with tempfile.TemporaryDirectory() as folder:
            output = Path(folder)
            existing = output / "chatgpt-01.txt"
            existing.write_text("synthetic previous export")
            args = SimpleNamespace(chrome_root=None, output_dir=output, proxy="", timeout=3,
                                   dry_run=False, replace=True)
            profile = SimpleNamespace(name="Synthetic", label="Synthetic")
            jar = CookieJar()
            jar.set_cookie(cookie("session", "synthetic"))
            with patch("scripts.import_chatgpt_cookies.parse_args", return_value=args), patch(
                "scripts.import_chatgpt_cookies.discover_profiles", return_value=[profile]
            ), patch("scripts.import_chatgpt_cookies.extract_chatgpt_cookies", return_value=jar), patch(
                "scripts.import_chatgpt_cookies.validate_session", return_value=(False, "HTTP 403")
            ):
                self.assertEqual(main(), 1)
            self.assertEqual(existing.read_text(), "synthetic previous export")

    def test_main_exports_only_verified_profiles_and_prunes_stale_numbered_files(self) -> None:
        with tempfile.TemporaryDirectory() as folder:
            output = Path(folder)
            (output / "chatgpt-02.txt").write_text("synthetic old file")
            args = SimpleNamespace(chrome_root=None, output_dir=output, proxy="", timeout=3,
                                   dry_run=False, replace=True)
            profiles = [SimpleNamespace(name="X Private", label="excluded"),
                        SimpleNamespace(name="Valid", label="valid"),
                        SimpleNamespace(name="Empty", label="empty"),
                        SimpleNamespace(name="Broken", label="broken")]
            jar = CookieJar()
            jar.set_cookie(cookie("session", "synthetic"))
            with patch("scripts.import_chatgpt_cookies.parse_args", return_value=args), patch(
                "scripts.import_chatgpt_cookies.discover_profiles", return_value=profiles
            ), patch("scripts.import_chatgpt_cookies.extract_chatgpt_cookies",
                     side_effect=[jar, CookieJar(), OSError("synthetic")]), patch(
                "scripts.import_chatgpt_cookies.validate_session", return_value=(True, "active")
            ):
                self.assertEqual(main(), 0)
            self.assertTrue((output / "chatgpt-01.txt").exists())
            self.assertFalse((output / "chatgpt-02.txt").exists())

    def test_dry_run_writes_nothing_and_no_profiles_fails(self) -> None:
        with tempfile.TemporaryDirectory() as folder:
            args = SimpleNamespace(chrome_root=None, output_dir=Path(folder), proxy="", timeout=3,
                                   dry_run=True, replace=True)
            jar = CookieJar()
            jar.set_cookie(cookie("session", "synthetic"))
            with patch("scripts.import_chatgpt_cookies.parse_args", return_value=args), patch(
                "scripts.import_chatgpt_cookies.discover_profiles",
                return_value=[SimpleNamespace(name="Valid", label="valid")]
            ), patch("scripts.import_chatgpt_cookies.extract_chatgpt_cookies", return_value=jar), patch(
                "scripts.import_chatgpt_cookies.validate_session", return_value=(True, "active")
            ):
                self.assertEqual(main(), 0)
            self.assertEqual(list(Path(folder).iterdir()), [])
            with patch("scripts.import_chatgpt_cookies.parse_args", return_value=args), patch(
                "scripts.import_chatgpt_cookies.discover_profiles", return_value=[]
            ):
                self.assertEqual(main(), 1)

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
