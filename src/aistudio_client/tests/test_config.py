from __future__ import annotations

import unittest
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest.mock import patch

from aistudio_client.config import Settings, parse_env, parse_netscape_cookie_header
from aistudio_client.cookie_files import discover_cookie_files


class ConfigTests(unittest.TestCase):
    def test_parses_quoted_env_values(self) -> None:
        values = parse_env("# comment\nA='one=two'\nB=three\n")
        self.assertEqual(values, {"A": "one=two", "B": "three"})

    def test_reads_only_matching_netscape_domains(self) -> None:
        text = "#HttpOnly_.google.com\tTRUE\t/\tTRUE\t0\tSAPISID\tvalue\nexample.com\tTRUE\t/\tTRUE\t0\tignored\tx\n"
        self.assertEqual(parse_netscape_cookie_header(text), "SAPISID=value")

    def test_discovers_cookie_files_in_natural_order(self) -> None:
        with TemporaryDirectory() as directory:
            root = Path(directory)
            for name in ("account-10.txt", "ignored.json", "account-2.txt"):
                (root / name).write_text("fixture", encoding="utf-8")

            names = [path.name for path in discover_cookie_files(root)]

        self.assertEqual(names, ["account-2.txt", "account-10.txt"])

    def test_settings_reads_first_cookie_from_cookie_directory(self) -> None:
        with TemporaryDirectory() as directory:
            root = Path(directory)
            cookies = root / "COOKIES"
            cookies.mkdir()
            (cookies / "account.txt").write_text(
                ".google.com\tTRUE\t/\tTRUE\t0\tSAPISID\tfirst",
                encoding="utf-8",
            )
            env_file = root / ".env"
            env_file.write_text("AISTUDIO_MODEL=models/test\n", encoding="utf-8")

            with patch.dict("os.environ", {"AISTUDIO_COOKIE_DIR": str(cookies)}):
                settings = Settings.load(env_file)

        self.assertEqual(settings.cookie_header, "SAPISID=first")
        self.assertEqual(settings.values["AISTUDIO_COOKIE_DIR"], str(cookies))

    def test_settings_can_run_without_mounted_env_file(self) -> None:
        with TemporaryDirectory() as directory:
            root = Path(directory)
            cookies = root / "cookies"
            cookies.mkdir()
            (cookies / "account.txt").write_text(
                ".google.com\tTRUE\t/\tTRUE\t0\tSAPISID\tenvironment",
                encoding="utf-8",
            )
            environment = {
                "AISTUDIO_COOKIE_DIR": str(cookies),
                "AISTUDIO_MODEL": "models/from-environment",
            }

            with patch.dict("os.environ", environment, clear=True):
                settings = Settings.load(root / "missing.env")

        self.assertEqual(settings.cookie_header, "SAPISID=environment")
        self.assertEqual(settings.model, "models/from-environment")
