from pathlib import Path
from tempfile import TemporaryDirectory
from unittest import TestCase

from browser_interface.cookie_revision import revision_matches, save_revision


class CookieRevisionTests(TestCase):
    def test_manual_cookie_replacement_invalidates_profile_revision(self):
        with TemporaryDirectory() as directory:
            root = Path(directory)
            cookie_file = root / "account.txt"
            profile = root / "profile"
            cookie_file.write_text("first", encoding="utf-8")

            save_revision(cookie_file, profile)
            self.assertTrue(revision_matches(cookie_file, profile))

            cookie_file.write_text("second", encoding="utf-8")
            self.assertFalse(revision_matches(cookie_file, profile))
