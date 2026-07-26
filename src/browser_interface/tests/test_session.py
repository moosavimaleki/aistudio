from pathlib import Path
from tempfile import TemporaryDirectory
from unittest import IsolatedAsyncioTestCase
from unittest.mock import AsyncMock, Mock

from browser_interface.browser.cookies import session_fingerprint
from browser_interface.browser.session import BrowserSession
from browser_interface.cookie_source import read_cookie_file


class BrowserSessionTests(IsolatedAsyncioTestCase):
    async def test_persistent_profile_is_reused_for_same_identity(self):
        session = BrowserSession(Mock(), "default", "http://chrome")
        session.context = AsyncMock()
        session.context.cookies.return_value = _auth_cookies("sid", "sapisid")
        fingerprint = session_fingerprint(
            "SID=sid; SAPISID=sapisid",
            "0",
        )

        self.assertTrue(await session._profile_matches(fingerprint, "0"))

    async def test_snapshot_updates_profile_cookie_file(self):
        with TemporaryDirectory() as directory:
            path = Path(directory) / "account.txt"
            profile = Path(directory) / "profile"
            session = BrowserSession(
                Mock(),
                "default",
                "http://chrome",
                path,
                profile,
            )
            session.context = AsyncMock()
            session.context.cookies.return_value = _auth_cookies("sid", "rotated")
            session.runtime_config = {"authUser": "0"}
            session.transport_profile = {"User-Agent": "Chrome"}
            session.auth_user = "0"

            result = await session.snapshot()

            self.assertIn("SAPISID=rotated", read_cookie_file(str(path)))
            self.assertIn(
                {"name": "SAPISID", "value": "rotated"},
                result["cookieRecords"],
            )
            self.assertTrue(session._revision_matches())


def _auth_cookies(sid: str, sapisid: str) -> list[dict]:
    return [
        _cookie("SID", sid, http_only=True),
        _cookie("SAPISID", sapisid),
    ]


def _cookie(name: str, value: str, *, http_only: bool = False) -> dict:
    return {
        "name": name,
        "value": value,
        "domain": ".google.com",
        "path": "/",
        "secure": True,
        "httpOnly": http_only,
        "expires": -1,
    }
