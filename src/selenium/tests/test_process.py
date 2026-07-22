import os
from pathlib import Path
from tempfile import TemporaryDirectory
from types import SimpleNamespace
from unittest import TestCase
from unittest.mock import patch

from selenium.browser.process import ChromeProcess


class ChromeProcessTests(TestCase):
    def test_profile_has_independent_paths_and_extension_config(self):
        with TemporaryDirectory() as directory:
            root = Path(directory)
            extension = root / "extension"
            (extension / "config").mkdir(parents=True)
            environment = {
                "CHROME_RUNTIME_DIR": str(root / "browsers"),
                "EXTENSION_SOURCE_DIR": str(extension),
                "FACTORY_ORIGIN": "http://factory:3345",
            }
            with patch.dict(os.environ, environment, clear=False):
                process = ChromeProcess(SimpleNamespace(browser_id="browser2"), 9224, 1)
                target = process._browser_path("extensions")
                target.mkdir(parents=True)
                process._write_extension_config(target)
                script = (target / "config" / "runtime-config.js").read_text()
                arguments = process._arguments(root, target)

        self.assertIn('"browserId": "browser2"', script)
        self.assertIn("http://factory:3345", script)
        self.assertIn("--remote-debugging-port=9224", arguments)

    def test_existing_profile_is_not_deleted(self):
        with TemporaryDirectory() as directory:
            profile = Path(directory) / "profile"
            profile.mkdir()
            marker = profile / "Local State"
            marker.write_text("device state", encoding="utf-8")
            lock = profile / "SingletonLock"
            lock.symlink_to("stale-process")

            ChromeProcess._ensure_directory(profile)
            ChromeProcess._remove_stale_locks(profile)

            self.assertEqual(marker.read_text(encoding="utf-8"), "device state")
            self.assertFalse(lock.exists())
