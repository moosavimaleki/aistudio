import os
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest import TestCase
from unittest.mock import patch

from browser_interface.config import load_browser_config


class BrowserConfigTests(TestCase):
    def test_indexed_cookie_files_create_independent_browsers(self):
        with TemporaryDirectory() as directory:
            root = Path(directory)
            for name in ("account-1.txt", "account-2.txt"):
                (root / name).write_text(
                    ".google.com\tTRUE\t/\tTRUE\t0\tSAPISID\ttest-value\n",
                    encoding="utf-8",
                )
            env_file = root / ".env"
            env_file.write_text("", encoding="utf-8")
            environment = {
                "AISTUDIO_ENV_FILE": str(env_file),
                "AISTUDIO_COOKIE_DIR": str(root),
                "CHROME_CDP_BASE_PORT": "9300",
            }
            with patch.dict(os.environ, environment, clear=True):
                config = load_browser_config()

        self.assertEqual(
            [browser.browser_id for browser in config.browsers],
            ["default", "browser2"],
        )
        self.assertEqual(config.cdp_base_port, 9300)
        self.assertEqual([browser.auth_user for browser in config.browsers], ["0", "0"])
        self.assertEqual(
            config.browsers[0].cookie_header,
            config.browsers[1].cookie_header,
        )
        self.assertEqual(config.browsers[0].cookie_file.name, "account-1.txt")
