import tempfile
import unittest
from pathlib import Path

from aistudio_client.config import Settings

from gencontent.profile_settings import ProfileSettings
from gencontent.profiles import BrowserProfile


class ProfileSettingsTests(unittest.TestCase):
    def test_second_profile_uses_second_cookie_file(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "first.txt").write_text(
                ".google.com\tTRUE\t/\tTRUE\t0\tSID\tfirst-session\n",
                encoding="utf-8",
            )
            cookie_file = root / "second.txt"
            cookie_file.write_text(
                ".google.com\tTRUE\t/\tTRUE\t0\tSID\tsecond-session\n",
                encoding="utf-8",
            )
            base = _settings(root)
            profile = BrowserProfile(2, "browser2", "0", True, True, None)

            selected = ProfileSettings(base).build(profile)

            self.assertEqual(selected.browser_id, "browser2")
            self.assertEqual(selected.auth_user, "0")
            self.assertEqual(selected.cookie_header, "SID=second-session")


def _settings(root: Path) -> Settings:
    return Settings(
        env_file=root / ".env",
        values={"AISTUDIO_COOKIE_DIR": str(root)},
        cookie_header="SID=first-session",
        origin_url="https://aistudio.google.com/prompts/new_chat",
        model="models/test",
        token_factory_url="http://browser-interface:3345/get-token",
        waa_api_key="test-key",
        proxy_url=None,
        auth_user="0",
        browser_id=None,
    )


if __name__ == "__main__":
    unittest.main()
